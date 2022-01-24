## Cloudflare Access SCIM handler

This is essentially a [SCIM](http://www.simplecloud.info) proxy to [Cloudflare Access](https://www.cloudflare.com/teams/access/).
It is developed and tested with [Rippling](https://www.rippling.com/) Custom App with SCIM support.

## Features
- create, list and delete groups
- add and remove users to groups
- simple caching

## Usage

- create `CLOUDFLARE_API_TOKEN` with permission to manage Teams.
- get `CLOUDFLARE_ACCESS_ACCOUNT_ID`
- choose a random `BEARER_TOKEN`

```shell
docker build -t cloudflare-scim .
export CLOUDFLARE_API_TOKEN='<TOKEN>'
export CLOUDFLARE_ACCESS_ACCOUNT_ID='<ACCOUNT_ID>'
export BEARER_TOKEN='<secret>'
docker run --rm -p 7643:7643 -e CLOUDFLARE_API_TOKEN -e CLOUDFLARE_ACCESS_ACCOUNT_ID -e BEARER_TOKEN
```

### Configure SCIM client

- set authentication to the chosen Bearer token
- use SCIM version 2.0
- allow using PATCH method to update group membership
- use user email as user principal name
- if an attribute is mandatory, use `externalId` with any value (ignored)
- if the SCIM client keeps track of user accounts (like Rippling), there should be a Teams group where all users are added,
otherwise created users will disappear after a sync

## Design

Cloudflare Teams use an email address as user principal name.
There is essentially no user database per se. The only notion of a user is as a member of a group.
Thus, when a SCIM client creates a user resource, it is cached locally and appears when listing all users.
The user is removed from cache when it is added to a group.

The Cloudflare API doesn't provide atomic updates (it doesn't use ETags nor the If-Unmodified-Since header).
Thus all the calls to Cloudflare API are serialized. Performance is not a concern.


## TODO

- test (the handler was developed and tested using end to end manual testing)
- documentation

## License

