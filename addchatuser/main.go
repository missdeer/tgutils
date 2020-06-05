package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Arman92/go-tdlib"
)

var (
	allChats            []*tdlib.Chat
	haveFullChatList    bool
	theChannelChatID    int64
	theSupergroupChatID int64
	theChannelID        int32
	theSupergroupID     int32
)

func main() {
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
			if group.IsChannel {
				fmt.Println("\tit's a channel")
				if strings.HasPrefix(chat.Title, `唯美和美食不可辜负-`) {
					theChannelChatID = chat.ID
					theChannelID = spChat.SupergroupID
					getSupergroupMemebers(client, theChannelID)
				}
			} else {
				fmt.Println("\tit's not a channel")
				if strings.HasPrefix(chat.Title, `唯美和美食不可辜负-`) {
					theSupergroupChatID = chat.ID
					theSupergroupID = spChat.SupergroupID
					getSupergroupMemebers(client, theSupergroupID)
				}
			}
			fullInfo, err := client.GetSupergroupFullInfo(spChat.SupergroupID)
			if err != nil {
				log.Println("can't get super group full info", err)
				break
			}
			if !fullInfo.CanGetMembers {
				log.Println("can't get members from this group", chat.Title)
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
}

func getSupergroupMemebers(client *tdlib.Client, supergroupID int32) {
	var offset int32 = 0
	var limit int32 = 200
	var filter tdlib.SupergroupMembersFilter
	m, err:=client.GetSupergroupMembers(supergroupID, filter, offset, limit)
	if err != nil {
		log.Println("getting supergroup member failed", err)
		return
	}
	for _, member := range m.Members{
		fmt.Println("find a member:", member.UserID)
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
