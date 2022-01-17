package cfscim

import (
	"fmt"
	"github.com/cloudflare/cloudflare-go"
	"github.com/elimity-com/scim"
	"github.com/elimity-com/scim/errors"
	"github.com/elimity-com/scim/optional"
	"github.com/scim2/filter-parser/v2"
	"io"
	"log"
	"net/http"
	"sort"
)

type cloudflareGroupResourceHandler struct{}

func (h cloudflareGroupResourceHandler) groupResource(group cloudflare.AccessGroup) scim.Resource {
	return scim.Resource{
		ID:         group.ID,
		ExternalID: optional.NewString(group.Name),
		Attributes: map[string]interface{}{
			"displayName": group.Name,
			"members":     h.getGroupMembers(group.Include),
		},
		Meta: scim.Meta{
			Created:      group.CreatedAt,
			LastModified: group.UpdatedAt,
			Version:      fmt.Sprintf("%d", group.UpdatedAt.Unix()),
		},
	}
}

func (h cloudflareGroupResourceHandler) Create(_ *http.Request, attributes scim.ResourceAttributes) (scim.Resource, error) {
	lock.Lock()
	defer lock.Unlock()

	log.Println(fmt.Sprintf("CreateGroup %s", attributes["displayName"]))

	accessGroup := cloudflare.AccessGroup{}
	accessGroup.Name = attributes["displayName"].(string)
	h.setMembers(&accessGroup, attributes)

	group, err := api.CreateAccessGroup(ctx, accountID, accessGroup)
	if err != nil {
		return scim.Resource{}, parseCloudflareError(err)
	}
	groupCache[group.ID] = group

	// return stored resource
	return h.groupResource(group), nil
}

func (h cloudflareGroupResourceHandler) Delete(_ *http.Request, id string) error {
	lock.Lock()
	defer lock.Unlock()

	log.Println(fmt.Sprintf("DeleteGroup: %s", id))

	err := api.DeleteAccessGroup(ctx, accountID, id)
	if err != nil {
		return parseCloudflareError(err)
	}
	delete(groupCache, id)
	return nil
}

func (h cloudflareGroupResourceHandler) Get(_ *http.Request, id string) (scim.Resource, error) {
	lock.Lock()
	defer lock.Unlock()

	log.Println(fmt.Sprintf("GetGroup: %s", id))

	cachedGroup, ok := groupCache[id]

	if ok {
		return h.groupResource(cachedGroup), nil
	}

	group, err := api.AccessGroup(ctx, accountID, id)
	if err != nil {
		return scim.Resource{}, errors.ScimErrorResourceNotFound(id)
	}

	// return resource with given identifier
	groupCache[id] = group
	return h.groupResource(group), nil
}

func (h cloudflareGroupResourceHandler) GetAll(_ *http.Request, params scim.ListRequestParams) (scim.Page, error) {
	lock.Lock()
	defer lock.Unlock()

	log.Println(fmt.Sprintf("GetAllGroups"))

	groups, err := getCloudflareGroups()
	if err != nil {
		return scim.Page{}, err
	}

	resources := []scim.Resource{}
	if params.Count == 0 || len(groups) == 0 {
		return scim.Page{TotalResults: len(groups), Resources: resources}, nil
	}

	sort.Sort(groupsByName(groups))

	i := 1

	for _, group := range groups {
		groupCache[group.ID] = group
		if i > (params.StartIndex + params.Count - 1) {
			continue
		}

		if i >= params.StartIndex {
			resources = append(resources, h.groupResource(group))
		}
		i++
	}

	return scim.Page{TotalResults: len(groups), Resources: resources}, nil
}

func (h cloudflareGroupResourceHandler) Patch(r *http.Request, id string, operations []scim.PatchOperation) (scim.Resource, error) {
	lock.Lock()
	defer lock.Unlock()

	log.Println(fmt.Sprintf("PatchGroup: %s", id))
	if body, err := io.ReadAll(r.Body); err == nil {
		log.Println(fmt.Sprintf("%s", body))
	}

	group, ok := groupCache[id]
	if !ok {
		temp, err := api.AccessGroup(ctx, accountID, id)
		if err != nil {
			return scim.Resource{}, parseCloudflareError(err)
		}
		group = temp
		groupCache[group.ID] = group
	}

	modified := false
	for _, op := range operations {
		switch op.Op {
		case scim.PatchOperationAdd:
			if op.Path != nil {
				if op.Path.String() == "members" {
					valueMap := op.Value.([]interface{})
					for _, v := range valueMap {
						if v, ok := v.(map[string]interface{}); ok {
							if v, ok := v["value"]; ok {
								if v, ok := v.(string); ok {
									add := true
									for _, i := range group.Include {
										if e, ok := i.(map[string]interface{}); ok {
											if e, ok := e["email"]; ok {
												if e, ok := e.(map[string]interface{}); ok {
													if e, ok := e["email"]; ok {
														if e, ok := e.(string); ok {
															if e == v {
																add = false
																break
															}
														}
													}
												}
											} else if e, ok := i.(cloudflare.AccessGroupEmail); ok {
												if e.Email.Email == v {
													add = false
													break
												}
											}

										}
									}
									if add {
										group.Include = append(group.Include, cloudflare.AccessGroupEmail{Email: struct {
											Email string `json:"email"`
										}{Email: v}})
										delete(userCache, v)
										modified = true
									}
								}
							}
						}
					}
				}
			}
		case scim.PatchOperationRemove:
			if op.Path.AttributePath.AttributeName == "members" {
				if ve, ok := op.Path.ValueExpression.(*filter.AttributeExpression); ok {
					if ve.AttributePath.AttributeName == "value" && ve.Operator == "eq" {
						if v, ok := ve.CompareValue.(string); ok {
							if removed := removeUser(&group, v); removed {
								modified = true
								delete(userCache, v)
							}
						}
					}
				} else {
					group.Include = []interface{}{}
					modified = true
				}
			}
		default:
			return scim.Resource{}, errors.ScimErrorBadParams([]string{fmt.Sprintf("op: %s", op.Op)})
		}
	}

	if modified {
		_group, err := api.UpdateAccessGroup(ctx, accountID, group)
		if err != nil {
			return scim.Resource{}, parseCloudflareError(err)
		}
		group = _group
		groupCache[group.ID] = group
	}

	// return resource with given identifier
	return h.groupResource(group), nil
}

func (h cloudflareGroupResourceHandler) Replace(r *http.Request, id string, attributes scim.ResourceAttributes) (scim.Resource, error) {
	lock.Lock()
	defer lock.Unlock()

	log.Println(fmt.Sprintf("ReplaceGroup: %s", id))
	if body, err := io.ReadAll(r.Body); err == nil {
		log.Println(fmt.Sprintf("%s", body))
	}

	accessGroup := cloudflare.AccessGroup{}
	accessGroup.ID = id
	accessGroup.Name = attributes["displayName"].(string)
	h.setMembers(&accessGroup, attributes)

	group, err := api.UpdateAccessGroup(ctx, accountID, accessGroup)
	if err != nil {
		return scim.Resource{}, parseCloudflareError(err)
	}

	groupCache[group.ID] = group

	// return resource with given identifier
	return h.groupResource(group), nil
}

func (h cloudflareGroupResourceHandler) setMembers(accessGroup *cloudflare.AccessGroup, attributes scim.ResourceAttributes) {
	if members, ok := attributes["members"].([]interface{}); ok {
		for _, rm := range members {
			m, ok := rm.(map[string]interface{})
			if !ok {
				continue
			}
			v, ok := m["value"]
			if !ok {
				continue
			}
			e, ok := v.(string)
			if !ok || e == "" {
				continue
			}
			accessGroup.Include = append(accessGroup.Include, cloudflare.AccessGroupEmail{Email: struct {
				Email string `json:"email"`
			}{Email: e}})
			delete(userCache, e)
		}
	}
}

func (h cloudflareGroupResourceHandler) getGroupMembers(groups []interface{}) []interface{} {
	r := []interface{}{}
	for _, e := range getGroupMemberList(groups) {
		r = append(r, map[string]string{
			"value": e,
		})
	}
	return r
}
