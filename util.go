package cfscim

import (
	"context"
	"fmt"
	"github.com/cloudflare/cloudflare-go"
	"github.com/elimity-com/scim/errors"
	"log"
	"os"
	"sort"
	"sync"
	"time"
)

var (
	api        *cloudflare.API
	accountID  string
	ctx        context.Context
	userCache  map[string]struct{}
	groupCache map[string]cloudflare.AccessGroup
	groupTS    time.Time
	lock       *sync.Mutex
	debug      bool
)

func NewCloudflare() {
	ctx = context.Background()

	_api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	if _, err = _api.VerifyAPIToken(ctx); err != nil {
		if cfErr, ok := err.(*cloudflare.APIRequestError); ok {
			log.Fatal(cfErr.Error())
		}
	}
	api = _api

	accountID = os.Getenv("CLOUDFLARE_ACCESS_ACCOUNT_ID")
	if accountID == "" {
		log.Fatal("invalid config. Access Account ID must not be empty")
	}

	lock = &sync.Mutex{}

	userCache = map[string]struct{}{}
	groupCache = map[string]cloudflare.AccessGroup{}

	debug = false
}

func getCloudflareGroups() ([]cloudflare.AccessGroup, error) {
	groups := []cloudflare.AccessGroup{}
	if time.Since(groupTS) < 60 {
		for _, group := range groupCache {
			groups = append(groups, group)
		}
	} else {
		// wipe cache
		groupCache = map[string]cloudflare.AccessGroup{}
		temp, ri, err := api.AccessGroups(ctx, accountID, cloudflare.PaginationOptions{})
		if err != nil {
			return nil, parseCloudflareError(err)
		}
		if ri.TotalPages > 1 {
			log.Println(fmt.Sprintf("AccessGroups returned %d total pages, but paging is not supported yet", ri.TotalPages))
			return nil, errors.ScimError{Status: 500, Detail: "paging Cloudflare API not supported yet"}
		}
		groups = temp

		for _, group := range groups {
			groupCache[group.ID] = group
		}
		groupTS = time.Now()
	}
	return groups, nil
}

func getGroupMemberList(groups []interface{}) (r []string) {
	for _, i := range groups {
		if t, ok := i.(map[string]interface{})["email"]; ok {
			if v, ok := t.(map[string]interface{})["email"]; ok {
				if e, ok := v.(string); ok {
					r = append(r, e)
				}
			}
		}
	}
	sort.Strings(r)
	return r
}

func removeUser(accessGroup *cloudflare.AccessGroup, v string) bool {
	newInclude := []interface{}{}
	for _, i := range accessGroup.Include {
		keep := true
		if e, ok := i.(map[string]interface{}); ok {
			if e, ok := e["email"]; ok {
				if e, ok := e.(map[string]interface{}); ok {
					if e, ok := e["email"]; ok {
						if e, ok := e.(string); ok {
							if e == v {
								keep = false
							}
						}
					}
				}
			}
		} else if e, ok := i.(cloudflare.AccessGroupEmail); ok {
			if e.Email.Email == v {
				keep = false
			}
		}
		if keep {
			newInclude = append(newInclude, i)
		}
	}
	if len(newInclude) < len(accessGroup.Include) {
		accessGroup.Include = newInclude
		return true
	}
	return false
}

func parseCloudflareError(err error) error {
	if cfErr, ok := err.(*cloudflare.APIRequestError); ok {
		return errors.ScimError{Status: cfErr.StatusCode, Detail: "Cloudflare error: " + cfErr.Error()}
	}
	return errors.ScimErrorInternal
}

type groupsByName []cloudflare.AccessGroup

func (g groupsByName) Len() int           { return len(g) }
func (g groupsByName) Less(i, j int) bool { return g[i].Name < g[j].Name }
func (g groupsByName) Swap(i, j int)      { g[i], g[j] = g[j], g[i] }
