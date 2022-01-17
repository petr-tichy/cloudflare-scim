package cfscim

import (
	"fmt"
	"github.com/elimity-com/scim"
	"github.com/elimity-com/scim/optional"
	"github.com/elimity-com/scim/schema"
	"github.com/gorilla/handlers"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
)

func handleAuth(next http.Handler) http.HandlerFunc {
	bearerToken := os.Getenv("BEARER_TOKEN")
	if bearerToken == "" {
		log.Fatal("invalid config. Bearer token must be set")
	}
	bearerToken = "Bearer " + bearerToken

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == bearerToken {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

func logHandler(next http.Handler) http.HandlerFunc {
	if false {
		return func(w http.ResponseWriter, r *http.Request) {
			x, err := httputil.DumpRequest(r, true)
			if err != nil {
				http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
				return
			}
			log.Println(fmt.Sprintf("%s", x))
			next.ServeHTTP(w, r)
		}
	} else {
		return func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		}
	}
}

func combinedLoggingHandler(next http.Handler) http.Handler {
	if true {
		return handlers.CombinedLoggingHandler(os.Stdout,
			next)
	} else {
		return next
	}
}

func ScimServer() {
	http.Handle("/scim/",
		combinedLoggingHandler(
			logHandler(
				http.StripPrefix("/scim",
					handleAuth(scim.Server{
						Config: scim.ServiceProviderConfig{
							SupportFiltering: false,
							SupportPatch:     false,
						},
						ResourceTypes: []scim.ResourceType{
							{
								ID:          optional.NewString("User"),
								Name:        "User",
								Endpoint:    "/Users",
								Description: optional.NewString("User Account"),
								Schema:      schema.CoreUserSchema(),
								Handler:     cloudflareUserResourceHandler{},
							},

							{
								ID:          optional.NewString("Group"),
								Name:        "Group",
								Endpoint:    "/Groups",
								Description: optional.NewString("Group"),
								Schema:      schema.CoreGroupSchema(),
								Handler:     cloudflareGroupResourceHandler{},
							},
						},
					})))))
	http.Handle("/", handlers.CombinedLoggingHandler(os.Stdout, http.NotFoundHandler()))
	log.Fatal(http.ListenAndServe(":7643", nil))
}
