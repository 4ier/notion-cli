# demo.tape — notion-cli demo GIF
Output demo.gif
Set Theme "Catppuccin Mocha"
Set FontFamily "JetBrains Mono"
Set FontSize 14
Set Width 960
Set Height 540
Set Padding 16
Set TypingSpeed 35ms
Set Shell "bash"

# Build first (hidden)
Hide
Type "cd ~/clawd/earn/notion-cli && export PATH=$PWD:$PATH"
Enter
Sleep 500ms
Show

# Main help
Type "notion"
Enter
Sleep 3s

# Database query help — shows filter syntax
Type "notion db query --help"
Enter
Sleep 4s

# Page commands
Type "notion page --help"
Enter
Sleep 3s

# Block commands
Type "notion block --help"
Enter
Sleep 3s

# Version
Type "notion --version"
Enter
Sleep 2s
