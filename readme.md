## gqlclient

This is a GraphQL Client for making requests to an endpoint. It provides a convenient interface for making GraphQL queries and mutations.

The Client has the following features:
- Endpoint - Specify the endpoint for the GraphQL requests
- HTTP Client - Specify the underlying HTTP client used for requests
- Token Provider - Provide a token provider for providing authentication tokens
- Retry Count - Specify the number of retries for requests

To create a Client, an Options struct must be provided. This struct contains the endpoint, HTTP client and token provider.

The Client provides two methods for making requests: Query and Mutation. These methods accept a query string and variables, and return a response.

The Variables parameter must be a map[string]interface{} or a struct.

If the request is unauthorized, the Client will attempt to refresh the token and retry the request up to the specified retry count.