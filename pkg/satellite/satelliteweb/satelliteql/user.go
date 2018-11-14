package satelliteql

import (
	"github.com/graphql-go/graphql"
)

const (
	userType = "user"

	fieldID        = "id"
	fieldEmail     = "email"
	fieldPassword  = "password"
	fieldFirstName = "firstName"
	fieldLastName  = "lastName"
)

// graphqlUser creates instance of user *graphql.Object
func graphqlUser() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: userType,
		Fields: graphql.Fields{
			fieldID: &graphql.Field{
				Type: graphql.String,
			},
			fieldEmail: &graphql.Field{
				Type: graphql.String,
			},
			fieldFirstName: &graphql.Field{
				Type: graphql.String,
			},
			fieldLastName: &graphql.Field{
				Type: graphql.String,
			},
		},
	})
}
