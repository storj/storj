Purpose of this is to process an API Key to see if it matches the correct client.

To use, run:
`$ go run process_api_key.go --key=yourApiKey`

Default api key is preset with the mocked headers. 

Where this is going:
We're going to be using macaroons to validate a token and permissions. This is a small step to building in that direction.