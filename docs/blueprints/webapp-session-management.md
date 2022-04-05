# Satellite web app session management 

## Abstract

This document details a new method for handling cookies and sessions for the satellite UI.

## Background

Currently, when a user logs into the satellite web app, the satellite stores a cookie in the client's browser containing a "[claim](https://github.com/storj/storj/blob/ee7f0d3a2ae71965c6e69baf61e4e7d1e967291c/satellite/console/consoleauth/claims.go#L17)" (user ID, email, expiration), which is signed by the satellite. The signature is included in the cookie.

When [authorizing an existing cookie](https://github.com/storj/storj/blob/ee7f0d3a2ae71965c6e69baf61e4e7d1e967291c/satellite/console/service.go#L1963), the satellite will 

* Validate the signature
* Ensure that the claim is not expired
* Get the user (based on user ID in claim) from the database
* Store the user struct from the DB, as well as the claim, in the request context ([link](https://github.com/storj/storj/blob/ee7f0d3a2ae71965c6e69baf61e4e7d1e967291c/satellite/console/auth.go#L54))

While this method of cookie authentication has worked so far, there is one major issue with it: we do not have a way to invalidate sessions from the backend. When a user logs out, the cookie is simply deleted from the client side, but this cookie is still valid for authentication. When a user resets their password, if they are logged in on some device already, this session is not invalidated. Basically, we have no way of managing or restricting existing sessions from the satellite backend. 

## Design

*1:* New table in the DB, named `web_app_sessions`:

* SessionID (blob, can be called ID)
* UserID (blob)
* IPAddress (string)
* UserAgent (string)
* CreatedAt (time)
* ExpiresAt (time)

*2:* The following interface to interact with the new table:

```
type WebappSessions interface {
    Create(ctx context.Context, userID uuid.UUID, ip, userAgent string, expires time.Duration) (WebappSession, error)
	GetBySessionID(ctx context.Context, sessionID uuid.UUID) (WebappSession, error)
	GetAllByUserID(ctx context.Context, userID uuid.UUID) ([]WebappSession, error)
    DeleteBySessionID(ctx context.Context, sessionID uuid.UUID) (error)
    DeleteAllByUserID(ctx context.Context, userID uuid.UUID) (int, error)
}

```

*3:* Integrate new table with the web app (and remove old Claim authentication):

*a) Logging in:*

When a user logs in, create a new session. Store the _signed_ session ID in a cookie, similarly to how we were storing a signed Claim

*b) Authorizing a request:*

Update the authorization middleware in server.go to validate the signature in the cookie, then retreive the session from the new table given the session ID in the cookie.

If the session is NotFound or ExpiresAt has passed, delete the cookie and do not authorize the request.

If the session is valid, retreive the User struct from the database using the UserID in the session. Store this in the context like before.

*c) Logging out:*

When calling the logout endpoint, before deleting the cookie, attempt to delete the session from the database


## Rationale

The primary disadvantage of the approach detailed here is that it requires an additional round-trip to the DB in order to authenticate a request. However, this is necessary if we ever want to implement security features involving the ability to invalidate specific sessions.

## Implementation

Whoops, this is covered in the *Design* section

## Wrapup

Once everything is working using the new session management design, remove unused code from our previous cookie authorization implementation.

The User Growth team is responsible for archiving this blueprint once completed.

