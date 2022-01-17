package cfscim

import (
	"fmt"
	"github.com/cloudflare/cloudflare-go"
	"github.com/elimity-com/scim"
	"github.com/elimity-com/scim/errors"
	"github.com/elimity-com/scim/optional"
	"github.com/mpvl/unique"
	"io"
	"log"
	"net/http"
)

type cloudflareUserResourceHandler struct{}

func (h cloudflareUserResourceHandler) Create(_ *http.Request, attributes scim.ResourceAttributes) (scim.Resource, error) {
	lock.Lock()
	defer lock.Unlock()

	id, ok := attributes["userName"].(string)
	if !ok {
		return scim.Resource{}, errors.ScimErrorBadParams([]string{"id"})
	}
	log.Println(fmt.Sprintf("CreateUser %s", id))

	userCache[id] = struct{}{}
	return scim.Resource{
		ID:         id,
		ExternalID: optional.NewString(id),
		Attributes: scim.ResourceAttributes{
			"userName": id,
			"active":   true,
		},
	}, nil
}

func (h cloudflareUserResourceHandler) Delete(_ *http.Request, id string) error {
	lock.Lock()
	defer lock.Unlock()

	log.Println(fmt.Sprintf("DeleteUser: %s", id))

	delete(userCache, id)

	return nil
}

func (h cloudflareUserResourceHandler) Get(_ *http.Request, id string) (scim.Resource, error) {
	log.Println(fmt.Sprintf("GetUser %s", id))

	return scim.Resource{
		ID:         id,
		ExternalID: optional.NewString(id),
		Attributes: scim.ResourceAttributes{
			"userName": id,
			"active":   true,
		},
	}, nil
}

func (h cloudflareUserResourceHandler) GetAll(_ *http.Request, params scim.ListRequestParams) (scim.Page, error) {
	lock.Lock()
	defer lock.Unlock()

	log.Println("GetAllUsers")

	accessGroups, err := getCloudflareGroups()
	if err != nil {
		return scim.Page{}, err
	}

	var members []string
	resources := []scim.Resource{}

	for _, g := range accessGroups {
		members = append(members, getGroupMemberList(g.Include)...)
	}

	for c := range userCache {
		members = append(members, c)
	}

	if len(members) == 0 {
		return scim.Page{
			TotalResults: 0,
			Resources:    resources,
		}, nil
	}

	// dedup
	unique.Sort(unique.StringSlice{P: &members})

	if params.Count == 0 {
		return scim.Page{
			TotalResults: len(members),
			Resources:    resources,
		}, nil
	}

	i := 1

	for _, id := range members {
		if i > (params.StartIndex + params.Count - 1) {
			break
		}

		if i >= params.StartIndex {
			resources = append(resources, scim.Resource{
				ID:         id,
				ExternalID: optional.NewString(id),
				Attributes: scim.ResourceAttributes{
					"userName": id,
					"active":   true,
				},
			})
		}
		i++
	}

	return scim.Page{
		TotalResults: len(members),
		Resources:    resources,
	}, nil
}

func (h cloudflareUserResourceHandler) Patch(_ *http.Request, id string, _ []scim.PatchOperation) (scim.Resource, error) {
	log.Println(fmt.Sprintf("PatchUser: %s", id))
	return scim.Resource{}, errors.ScimError{
		Status: http.StatusNotImplemented,
	}
}

func (h cloudflareUserResourceHandler) Replace(r *http.Request, id string, attributes scim.ResourceAttributes) (scim.Resource, error) {
	lock.Lock()
	defer lock.Unlock()

	log.Println(fmt.Sprintf("ReplaceUser: %s", id))
	if body, err := io.ReadAll(r.Body); err == nil && debug {
		log.Println(fmt.Sprintf("%s", body))
	}

	active := true
	if v, ok := attributes["active"]; ok {
		v, ok := v.(bool)
		if !ok {
			return scim.Resource{}, errors.ScimErrorInvalidValue
		}
		active = v
	}

	if !active {
		if err := h.deactivate(id); err != nil {
			return scim.Resource{}, parseCloudflareError(err)
		}
		api.RevokeAccessUserTokens(ctx, accountID, cloudflare.AccessUserEmail{Email: id})
	}

	return scim.Resource{
		ID:         id,
		ExternalID: optional.NewString(id),
		Attributes: scim.ResourceAttributes{
			"userName": id,
			"active":   active,
		},
	}, nil
}

func (h cloudflareUserResourceHandler) deactivate(id string) error {
	groups, err := getCloudflareGroups()
	if err != nil {
		return err
	}

	for _, accessGroup := range groups {
		if removeUser(&accessGroup, id) {
			group, err := api.UpdateAccessGroup(ctx, accountID, accessGroup)
			if err != nil {
				return parseCloudflareError(err)
			}
			groupCache[group.ID] = group
		}
	}

	return nil
}
