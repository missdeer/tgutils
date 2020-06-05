# tgutils

## Dependencies

Clone the Tdlib repo and build it:
```bash
git clone git@github.com:tdlib/td.git --depth 1
cd td
mkdir build
cd build
cmake -DCMAKE_BUILD_TYPE=Release ..
cmake --build . -- -j5
make install

# -j5 refers to number of your cpu cores + 1 for multi-threaded build.
```

Maybe you need:

```bash
cmake -DCMAKE_BUILD_TYPE=Release -DOPENSSL_ROOT_DIR=/usr/local/opt/openssl/ -DOPENSSL_LIBRARIES=/usr/local/opt/openssl/lib/ ..
```

on macOS.

If hit any build errors, refer to [Tdlib build instructions](https://github.com/tdlib/td#building)