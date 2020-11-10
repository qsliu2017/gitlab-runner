Write-Output "Fixing bad symlink to C:\GitLab-Runner"

(Get-Item C:\GitLab-Runner).Delete()

# Ensure the symlink contains a trailing backslash
New-Item -ItemType SymbolicLink -Path "C:\GitLab-Runner" -Target "\\vboxsvr\C:_GitLab-Runner\"
