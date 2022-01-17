package main

import (
	"github.com/petr-tichy/cloudflare-scim"
)

func main() {
	cfscim.NewCloudflare()
	cfscim.ScimServer()
}
