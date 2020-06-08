package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Arman92/go-tdlib"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/mssql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

var (
	searchOnly          bool
	largeGroupOnly      bool
	dbDriver            string
	dbConn              string
	allChats            []*tdlib.Chat
	haveFullChatList    bool
	theChannelChatID    int64
	theSupergroupChatID int64
	theChannelID        int32
	theSupergroupID     int32
	db                  *gorm.DB
)

const limit int32 = 200

type TGUser struct {
	gorm.Model
	UserID   int32  `gorm:"unique;index;not null"`
	UserName string `gorm:"type:varchar(255)"`
}

func userExists(userID int32) bool {
	u := &TGUser{UserID: userID}
	err := db.Where("user_id = ?", userID).First(u).Error
	return !gorm.IsRecordNotFoundError(err)
}

func insertUser(userID int32, userName string) error {
	u := &TGUser{UserID: userID, UserName: userName}
	err := db.Create(u).Error
	return err
}

func insertUserIfNotExists(client *tdlib.Client, userID int32) error {
	user, err := client.GetUser(userID)
	if err != nil {
		log.Println("can't get user info:", err)
		return err
	}

	if userExists(userID) {
		return errors.New("User exists")
	}

	if err = insertUser(userID, user.Username); err != nil {
		log.Println("insert user failed", err)
		return err
	}
	return nil
}

func main() {
	flag.StringVar(&dbDriver, "driver", "", "database driver, such as mysql, sqlite, postgres, mssql")
	flag.StringVar(&dbConn, "connection", "", "database connection string, for example: root:password7@tcp(192.168.233.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local")
	flag.BoolVar(&largeGroupOnly, "largeGroupOnly", largeGroupOnly, "large group (member count > 10000) only, or it will search all groups")
	flag.BoolVar(&searchOnly, "searchOnly", searchOnly, "search only, or it will get recent 10000 members")
	flag.Parse()

	var err error
	if db, err = gorm.Open(dbDriver, dbConn); err != nil {
		log.Fatal("failed to connect database", err)
	}
	defer db.Close()

	// Migrate the schema
	db.AutoMigrate(&TGUser{})

	tdlib.SetLogVerbosityLevel(1)
	tdlib.SetFilePath("./errors.txt")

	// Create new instance of client
	client := tdlib.NewClient(tdlib.Config{
		APIID:               "187786",
		APIHash:             "e782045df67ba48e441ccb105da8fc85",
		SystemLanguageCode:  "en",
		DeviceModel:         "Server",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "1.0.0",
		UseMessageDatabase:  true,
		UseFileDatabase:     true,
		UseChatInfoDatabase: true,
		UseTestDataCenter:   false,
		DatabaseDirectory:   "./tdlib-db",
		FileDirectory:       "./tdlib-files",
		IgnoreFileNames:     false,
	})

	// Handle Ctrl+C , Gracefully exit and shutdown tdlib
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		client.DestroyInstance()
		os.Exit(1)
	}()

	for {
		currentState, _ := client.Authorize()
		if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitPhoneNumberType {
			fmt.Print("Enter phone: ")
			var number string
			fmt.Scanln(&number)
			_, err := client.SendPhoneNumber(number)
			if err != nil {
				fmt.Printf("Error sending phone number: %v", err)
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitCodeType {
			fmt.Print("Enter code: ")
			var code string
			fmt.Scanln(&code)
			_, err := client.SendAuthCode(code)
			if err != nil {
				fmt.Printf("Error sending auth code : %v", err)
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitPasswordType {
			fmt.Print("Enter Password: ")
			var password string
			fmt.Scanln(&password)
			_, err := client.SendAuthPassword(password)
			if err != nil {
				fmt.Printf("Error sending auth password: %v", err)
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateReadyType {
			fmt.Println("Authorization Ready! Let's rock")
			break
		}
	}

	fmt.Println("authorized")
	// get at most 1000 chats list
	getChatList(client, 1000)
	fmt.Printf("got %d chats\n", len(allChats))

	for _, chat := range allChats {
		switch chat.Type.GetChatTypeEnum() {
		case tdlib.ChatTypeSupergroupType:
			spChat, ok := chat.Type.(*tdlib.ChatTypeSupergroup)
			if !ok {
				log.Println("can't convert to super group")
				break
			}
			group, err := client.GetSupergroup(spChat.SupergroupID)
			if err != nil {
				log.Println("can't get super group", err)
				break
			}
			fmt.Print("super group:", chat.Title, group.MemberCount, group.Username, chat.ID)
			fullInfo, err := client.GetSupergroupFullInfo(spChat.SupergroupID)
			if err != nil {
				log.Println("can't get super group full info", err)
				break
			}
			if !fullInfo.CanGetMembers {
				log.Println("can't get members from this group", chat.Title)
				break
			}
			if group.IsChannel {
				fmt.Println("\tit's a channel")
				if strings.HasPrefix(chat.Title, `唯美和美食不可辜负-`) {
					theChannelChatID = chat.ID
					theChannelID = spChat.SupergroupID
				} else {
					if !searchOnly {
						if fullInfo.MemberCount > 10000 || !largeGroupOnly {
							getSupergroupMemebers(client, spChat.SupergroupID)
						}
					}
					if fullInfo.MemberCount > 10000 {
						getChatMembers(client, chat.ID)
					}
				}
			} else {
				fmt.Println("\tit's not a channel")
				if strings.HasPrefix(chat.Title, `唯美和美食不可辜负-`) {
					theSupergroupChatID = chat.ID
					theSupergroupID = spChat.SupergroupID
				} else {
					if !searchOnly {
						if fullInfo.MemberCount > 10000 || !largeGroupOnly {
							getSupergroupMemebers(client, spChat.SupergroupID)
						}
					}
					if fullInfo.MemberCount > 10000 {
						getChatMembers(client, chat.ID)
					}
				}
			}
		case tdlib.ChatTypeBasicGroupType:
			basicChat, ok := chat.Type.(*tdlib.ChatTypeBasicGroup)
			if !ok {
				log.Println("can't convert to basic group")
				break
			}
			group, err := client.GetBasicGroup(basicChat.BasicGroupID)
			if err != nil {
				log.Println("can't get basic group", err)
				break
			}
			fmt.Println("basic group:", chat.Title, group.MemberCount)
			fullInfo, err := client.GetBasicGroupFullInfo(basicChat.BasicGroupID)
			if err != nil {
				log.Println("can't get super group full info", err)
				break
			}
			members := fullInfo.Members
			for _, member := range members {
				fmt.Println("user id:", member.UserID, "in", chat.Title)
			}
		}
	}

	rawUpdates := client.GetRawUpdatesChannel(100)
	for update := range rawUpdates {
		// Show all updates
		fmt.Println(update.Data)
		t, ok := update.Data["@type"]
		if !ok {
			continue
		}
		msgType, ok := t.(string)
		if !ok {
			continue
		}

		if msgType == "updateUserStatus" {
			id, ok := update.Data["user_id"]
			if !ok {
				continue
			}

			senderUserID, ok := id.(int32)
			if !ok {
				continue
			}

			insertUserIfNotExists(client, senderUserID)
			continue
		}

		if msgType != "updateNewMessage" && msgType != "updateChatLastMessage" {
			continue
		}
		c, ok := update.Data["content"]
		if !ok {
			continue
		}

		content, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		id, ok := content["sender_user_id"]
		if !ok {
			continue
		}

		senderUserID, ok := id.(int32)
		if !ok {
			continue
		}

		insertUserIfNotExists(client, senderUserID)
	}
}

func addMembers(client *tdlib.Client, m *tdlib.ChatMembers) error {
	if m.TotalCount == 0 || len(m.Members) == 0 {
		log.Println("got 0 members")
		return errors.New("no members")
	}
	fmt.Println("total count:", m.TotalCount, ", got member count:", len(m.Members))
	for _, member := range m.Members {
		user, err := client.GetUser(member.UserID)
		if err != nil {
			log.Println("can't get user info:", err)
			continue
		}
		if userExists(member.UserID) {
			continue
		}
		if err := insertUser(member.UserID, user.Username); err != nil {
			log.Println("insert user failed", err)
		}
	}
	return nil
}

func getChatMembers(client *tdlib.Client, chatID int64) {
	str := `0123456789abcdefghijklmnopqrstuvwxyz`
	var filter tdlib.ChatMembersFilter
	for _, ch := range str {
		searchStr := fmt.Sprintf("%c", ch)
		m, err := client.SearchChatMembers(chatID, searchStr, limit, filter)
		if err != nil {
			log.Println("getting supergroup member failed", err)
			continue
		}

		if err = addMembers(client, m); err != nil {
			log.Println("", err)
		}
		time.Sleep(time.Duration(len(m.Members)/60+1) * time.Second)
	}
}

func getSupergroupMemebers(client *tdlib.Client, supergroupID int32) {
	var offset int32 = 0
	var filter tdlib.SupergroupMembersFilter
	for ; ; offset += limit {
		m, err := client.GetSupergroupMembers(supergroupID, filter, offset, limit)
		if err != nil {
			log.Println("getting supergroup member failed", err)
			break
		}

		if err = addMembers(client, m); err != nil {
			log.Println("", err)
			break
		}
		time.Sleep(time.Duration(len(m.Members)/60+1) * time.Second)
	}
}

// see https://stackoverflow.com/questions/37782348/how-to-use-getchats-in-tdlib
func getChatList(client *tdlib.Client, limit int) error {
	if !haveFullChatList && limit > len(allChats) {
		offsetOrder := int64(math.MaxInt64)
		offsetChatID := int64(0)
		var lastChat *tdlib.Chat

		if len(allChats) > 0 {
			lastChat = allChats[len(allChats)-1]
			offsetOrder = int64(lastChat.Order)
			offsetChatID = lastChat.ID
		}

		// get chats (ids) from tdlib
		chats, err := client.GetChats(tdlib.JSONInt64(offsetOrder),
			offsetChatID, int32(limit-len(allChats)))
		if err != nil {
			return err
		}
		if len(chats.ChatIDs) == 0 {
			haveFullChatList = true
			return nil
		}

		for _, chatID := range chats.ChatIDs {
			// get chat info from tdlib
			chat, err := client.GetChat(chatID)
			if err == nil {
				allChats = append(allChats, chat)
			} else {
				return err
			}
		}
		return getChatList(client, limit)
	}
	return nil
}
