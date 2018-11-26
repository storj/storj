import x from '../apolloManager';
import gql from "graphql-tag";

// Performs graqhQL request.
// Returns Token, User objects.
// Throws an exception if error occurs
export async function login(email: string, password: string): Promise<any> {
    let response = await x.query({
        query: gql(`
                        query {
                            token(email: "${email}",
                            password: "${password}") {
                                token,
                                user{
                                    id,
                                    firstName,
                                    lastName,
                                    email,
                                    company{
                                        name,
                                        address,
                                        country,
                                        city,
                                        state,
                                        postalCode
                                    }
                                }
                            }
                        }`),
        fetchPolicy: "no-cache",
    });

    console.log(response);
    if (!response) {
        console.error("No token received");

        return null;
    }

    return response;
}