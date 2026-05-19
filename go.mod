module github.com/LeeFred3042U/kitcat

go 1.25.9

require golang.org/x/term v0.43.0

require golang.org/x/sys v0.44.0 // indirect

// Windows-only dependency (keychain backend).
require github.com/danieljoos/wincred v1.2.3
