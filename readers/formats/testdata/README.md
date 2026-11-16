# Archive fixtures

`board.odb.7z` and `board-encrypted.odb.7z` hold the ODB++ job in `../../odb/testdata/board`, packed
one folder down under `board/`, as producers wrap a job. 7-Zip itself made them, so the 7z decoder is
tested against the tool's own output (agni issue 1591). 7-Zip 26.06 made both, from a copy of the
folder with no macOS `._` files:

```
7zz a -t7z -ms=on -mx=9 board.odb.7z board
7zz a -t7z -ms=on -mhe=on -pagni-fixture board-encrypted.odb.7z board
```

The first is solid. The second encrypts the headers too, so its file list needs the password as well.
Regenerate both when the board job changes.
