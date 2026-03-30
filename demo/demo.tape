Output demo/demo.gif

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set Theme "Dracula"

Type `MUXWARP_CONFIG=demo/muxwarp.config.yaml SSH_OPTIONS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i demo/id_ed25519" ./bin/muxwarp`
Enter
Sleep 4s

Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms

Type "/"
Sleep 500ms
Type "api"
Sleep 1500ms
Escape
Sleep 500ms

Type "t"
Sleep 1s
Enter
Sleep 1500ms
Type "t"
Sleep 500ms

Type "q"
Sleep 500ms
