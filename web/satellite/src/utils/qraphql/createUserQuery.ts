import x from '../apolloManager';
import gql from "graphql-tag";

// Performs graqhQL request.
// Throws an exception if error occurs
export async function createUser(user: User, password: string): Promise<any> {

    let response = x.mutate(
        {
            mutation: gql(`
       mutation {
        createUser(
            input:{
                email: "${user.email}",
                password: "${password}",
                firstName: "${user.firstName}",
                lastName: "${user.lastName}",
                company: {
                    name: "${user.company.name}",
                    address: "${user.company.address}",
                    country: "${user.company.country}",
                    city: "${user.company.city}",
                    state: "${user.company.state}",
                    postalCode: "${user.company.postalCode}"
                }
            }
        )
       }
       `),
            fetchPolicy: "no-cache",
        }
    );

    if(!response){
        console.log("cannot create user")

        return null;
    }

    return response;
}

