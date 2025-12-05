# Session Management Testplan

&nbsp;

## Background
This testplan covers security must haves regarding session management 

Here is the [Session Management Blueprint](https://github.com/storj/storj/commit/3397886) which describes a design providing the satellite greater
control over sessions authorized to use the web app

&nbsp;

&nbsp;


| Test Scenario     | Test Case              | Description                                                                                                                                                                                                                                            | Comments |   
|-------------------|------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|
| Cookie Handling   | Session ID             | When user logs in and creates a new session, there should be a signed session ID in a cookie                                                                                                                                                           |          |   
|                   | Request Authorization  | With a signed session ID in a cookie, there should be a request to recieve the session from the WebappSessions table w/ the given session ID in the cookie                                                                                             |          |   
|                   | Authorization 1        | First the session checked if NotFound or ExpiresAt has passed from the new table given the session ID in the cookie, then the cookie is deleted and request is not authorized                                                                          |          |   
|                   | Authorization 2        | Second, if there were no issues with authorization from the previous step and if the session is valid from the new table given the session ID in the cookie, then the User struct should be received from the database using the UserID in the session |          |   
|                   | Logout- Delete Session | If logout endpoint is called, then session should be deleted from database                                                                                                                                                                             |          |   
|                   | Cookie Deletion        | Cookie should be deleted only after session is deleted from database                                                                                                                                                                                   |          |   
|                   | Inactivity Timout      | Session should be invalidated if user is inactive for a set period of time                                                                                                                                                                             |          |   
|                   | Closing the Browser    | A session should be invalidated once the user closes their browser for a session                                                                                                                                                                       |          |   
|                   | New IP Address         | If a session is created from a new IP(different than previous session), then there should be two-factor authentication to verify the user                                                                                                              |          |   
|                   | Valid Authorization    | If the session is valid from the new table given the session ID in the cookie, then User struct should be received from the database using the UserID in the session                                                                                   |          |   
| Multiple Sessions | Logout- Delete Session | If a user is logged into multiple sessions, then only the current session should be invalidated once the user logs out from said current session                                                                                                                    |          |   
|                   | Reset Password         | If a user resets their password, then all sessions should be invalidated once the user successfully resets their password                                                                                                                              |          |   
|                   | Inactivity Timeout     | Regardless of how many sessions there are, if there is inactivity in one session then only that session should be invalidated for security purposes                                                                                                        |          |   
|                   | Closing the Browser    | Regardless of how many sessions there are, only the current session should be invalidated once the user closes the browser used for said session                                                                                                                       |          |   
