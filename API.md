# API Documentation

> **Interactive reference:** [live API reference](https://feedsanctum.onrender.com/docs) or [local](http://localhost:8081/docs)

## Endpoints Summary

### Authentication & Users
| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| `POST` | [`/api/v1/register`](#register-user) | Create a new user account | No |
| `POST` | [`/api/v1/login`](#login-user) | Authenticate and obtain tokens | No |
| `POST` | [`/api/v1/refresh`](#refresh-tokens) | Rotate refresh token and get a new access token | Yes (Refresh Token) |
| `POST` | [`/api/v1/logout`](#logout-user) | Revoke a refresh token | Yes (Refresh Token) |
| `GET` | [`/api/v1/me`](#get-current-user) | Fetch the authenticated user's profile | Yes (Access Token) |
| `PATCH` | [`/api/v1/me`](#update-current-user) | Partially update the authenticated user's profile | Yes (Access Token) |
| `DELETE` | [`/api/v1/me`](#delete-current-user) | Delete the authenticated user account | Yes (Access Token) |

### Feeds
| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| `GET` | [`/api/v1/feeds`](#get-all-feeds) | Retrieve all registered RSS feeds | No |
| `GET` | [`/api/v1/feeds/{feedID}`](#get-feed-by-id) | Retrieve a specific feed by its ID | No |
| `POST` | [`/api/v1/feeds`](#create-feed) | Register a new RSS feed and automatically follow it | Yes (Access Token) |
| `GET` | [`/api/v1/follows`](#get-feed-follows) | Retrieve all feeds followed by the authenticated user | Yes (Access Token) |
| `POST` | [`/api/v1/follows`](#follow-feed) | Follow a feed by the authenticated user| Yes (Access Token) |
| `DELETE` | [`/api/v1/follows/{feedID}`](#delete-feed-follow) | Unfollow a feed for the authenticated user | Yes (Access Token) |

### Posts
| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| `GET` | [`/api/v1/posts`](#get-posts-for-user) | Retrieve all posts for feeds followed by the user | Yes (Access Token) |
| `POST` | [`/api/v1/posts/{postID}/read`](#mark-post-as-read) | Mark a specific post as read for the user | Yes (Access Token) |
| `GET` | [`/api/v1/posts/read`](#get-read-posts-for-user) | Retrieve all posts marked as read by the user | Yes (Access Token) |

## Rate Limiting

All endpoints are rate-limited by client IP and endpoint path. Authentication endpoints have additional protection:

| Scope | Limit | Window |
| :--- | :--- | :--- |
| All endpoints (per IP + endpoint) | 60 requests | 1 minute |
| `POST /api/v1/login` (per IP) | 5 requests | 1 minute |
| `POST /api/v1/login` (per email) | 8 requests | 1 minute |

Exceeding any limit returns:

##### `429 Too Many Requests`
```json
{ "error": "Rate limit reached" }
```
---

## Authentication & Users

### Authentication

All protected routes require `Authorization: Bearer <token>` in the request header.

- **Access token:** obtained from `POST /api/v1/login`. Short-lived (1 hour). Used on all `/me`, `/feeds` (POST), `/follows`, and `/posts` endpoints.
- **Refresh token:** obtained from `POST /api/v1/login`. Used only on `POST /api/v1/refresh` and `POST /api/v1/logout` to rotate or revoke the session.

To authenticate in Swagger UI, click **Authorize** and paste both tokens into their respective fields.

---

### Register User

Creates a new user account with a hashed password.

* **URL:** `/api/v1/register`
* **Method:** `POST`
* **Authentication Required:** No (Public)
* **Headers:**
  * `Content-Type: application/json`

---

#### Request Body

| Field | Type | Required | Constraints | Description |
| :--- | :--- | :--- | :--- | :--- |
| `name` | string | Yes | 1–45 chars, no control characters | User's display name. Leading/trailing whitespace is trimmed. |
| `email` | string | Yes | 1–50 chars, valid email format | Unique email address. Checked via RFC 5322 parsing. |
| `password` | string | Yes | 20–100 chars | Plaintext password to hash. |

**Example Request:**

```json
{
  "name": "Jane Doe",
  "email": "jane@example.com",
  "password": "correct horse battery staple 123"
}
```
#### Responses

##### `201 Created`
Returned when the user is successfully created.

```json
{
  "id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
  "name": "Jane Doe",
  "email": "jane@example.com",
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:00Z"
}
```

##### `400 Bad Request`
Returned when the JSON payload is malformed or input validation fails.

*Malformed JSON:*
```json
{
  "error": "invalid request payload"
}
```

*Validation Errors:*
```json
{
  "errors": [
    {
      "field": "email",
      "message": "invalid email format"
    }
  ]
}
```

##### `409 Conflict`
Returned when the email address is already registered.

```json
{
  "error": "email already registered"
}
```

##### `500 Internal Server Error`
Returned when an unhandled server error occurs (e.g., password hashing failure or database connection failure).

```json
{
  "error": "something went wrong"
}
```
---

### Login User

Authenticates a user with email and password, returning an access token (JWT) and a refresh token.

* **URL:** `/api/v1/login`
* **Method:** `POST`
* **Authentication Required:** No (Public)
* **Headers:**
  * `Content-Type: application/json`

---

#### Request Body

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `email` | string | Yes | Registered user email. |
| `password` | string | Yes | Plaintext user password. |

**Example Request:**

```json
{
  "email": "jane@example.com",
  "password": "superlongsupersecurepassword123"
}
```

---

#### Responses

##### `200 OK`
Returned on successful authentication with user details and tokens.

```json
{
  "id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
  "name": "Jane Doe",
  "email": "jane@example.com",
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:00Z",
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "f47ac10b-58cc-4372-a567-0e02b2c3d479"
}
```

##### `400 Bad Request`
Returned when the JSON request body is malformed.

```json
{
  "error": "invalid request payload"
}
```

##### `401 Unauthorized`
Returned when the email does not exist or the password does not match.

```json
{
  "error": "wrong email or password"
}
```

##### `500 Internal Server Error`
Returned when token generation fails.

```json
{
  "error": "failed to issue token"
}
```
or database token storage fails.

```json
{
    "error": "failed to save refresh token"
}
```
---

### Refresh Tokens

Rotates an existing, valid refresh token and issues a new access token (JWT) along with a replacement refresh token.

* **URL:** `/api/v1/refresh`
* **Method:** `POST`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <refresh_token>`

---

#### Request Body
None.

---

#### Responses

##### `200 OK`
Returned when tokens are successfully rotated.

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "3a7b18e4-84bf-4dc7-b248-cb5897c5f899"
}
```

##### `400 Bad Request`
Returned when the `Authorization` header is missing, malformed, or does not contain a Bearer token.

```json
{
  "error": "failed to get bearer token"
}
```

##### `401 Unauthorized`
Returned when the refresh token does not exist, has expired, or is invalid.

```json
{
  "error": "failed to find token"
}
```

##### `500 Internal Server Error`
Returned when token generation, rotation, or database storage fails.

```json
{
  "error": "failed to issue token"
}
```
---

### Logout User

Revokes the provided refresh token so it can no longer be used to issue new access tokens.

* **URL:** `/api/v1/logout`
* **Method:** `POST`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <refresh_token>`

---

#### Request Body
None.

---

#### Responses

##### `204 No Content`
Returned when the refresh token has been successfully revoked. No response body is returned.

##### `400 Bad Request`
Returned when the `Authorization` header is missing, malformed, or does not contain a Bearer token.

```json
{
  "error": "failed to get bearer token"
}
```

##### `500 Internal Server Error`
Returned when a database error occurs while revoking the token.

```json
{
  "error": "failed to revoke token"
}
```

---

### Get Current User

Retrieves the profile information of the currently authenticated user.

* **URL:** `/api/v1/me`
* **Method:** `GET`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`

---

#### Request Body
None.

---

#### Responses

##### `200 OK`
Returned with the authenticated user's details.

```json
{
  "id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
  "name": "Jane Doe",
  "email": "jane@example.com",
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:00Z"
}
```

##### `401 Unauthorized`
Returned by the auth middleware when the access token is missing, expired, or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `404 Not Found`
Returned when the authenticated user ID does not match any record in the database.

```json
{
  "error": "user not found"
}
```

##### `500 Internal Server Error`
Returned when an internal server error occurs (such as a database failure or missing context).

*Database Error:*
```json
{
  "error": "failed to get user"
}
```

*Context Error:*
```json
{
  "error": "missing user id in context"
}
```

---

### Update Current User

Partially updates the authenticated user's profile details. At least one field must be provided.

* **URL:** `/api/v1/me`
* **Method:** `PATCH`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`
  * `Content-Type: application/json`

---

#### Request Body

| Field | Type | Required | Constraints | Description |
| :--- | :--- | :--- | :--- | :--- |
| `name` | string | Optional | 1–45 chars, no control characters | New display name. |
| `email` | string | Optional | 1–50 chars, valid email format | New email address (must be unique). |
| `password` | string | Optional | 20–100 chars | New plaintext password to hash and update. |

*Note: The request body cannot be empty; at least one field must be specified.*

**Example Request:**

```json
{
  "name": "Jane Smith",
  "email": "janesmith@example.com"
}
```

---

#### Responses

##### `200 OK`
Returned when user details are successfully updated.

```json
{
  "id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
  "name": "Jane Smith",
  "email": "janesmith@example.com",
  "updated_at": "2025-01-15T11:45:00Z"
}
```

##### `400 Bad Request`
Returned when the JSON payload is malformed, no fields are provided, or input validation fails.

*Malformed JSON:*
```json
{
  "error": "invalid request payload"
}
```

*No Fields Provided:*
```json
{
  "error": "no fields to update"
}
```

*Validation Errors:*
```json
{
  "error": "invalid name"
}
```
*(or `"invalid email"`, `"invalid password"`)*

##### `401 Unauthorized`
Returned by auth middleware when the access token is missing or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `409 Conflict`
Returned when attempting to update to an email address that is already registered by another account.

```json
{
  "error": "email already registered"
}
```

##### `500 Internal Server Error`
Returned on internal server errors, context lookup issues, database failures, or hashing failure.

*Database Failure:*
```json
{
  "error": "something went wrong"
}
```

*Password Hashing Failure:*
```json
{
  "error": "failed to hash password"
}
```

*Context Lookup Failure:*
```json
{
  "error": "missing user id in context"
}
```

---

### Delete Current User

Deletes the authenticated user's account and associated data from the system.

* **URL:** `/api/v1/me`
* **Method:** `DELETE`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`

---

#### Request Body
None.

---

#### Responses

##### `204 No Content`
Returned when the user account is successfully deleted. No response body is returned.

##### `401 Unauthorized`
Returned by auth middleware when the access token is missing, expired, or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `500 Internal Server Error`
Returned when an error occurs while deleting the user from the database or retrieving context.

*Database Error:*
```json
{
  "error": "failed to delete user"
}
```

*Context Error:*
```json
{
  "error": "missing user id in context"
}
```

---

## Feeds

### Get All Feeds

Retrieves a list of all registered RSS feeds in the system.

* **URL:** `/api/v1/feeds`
* **Method:** `GET`
* **Authentication Required:** No (Public)
* **Headers:** None

---

#### Request Body
None.

---

#### Responses

##### `200 OK`
Returned with a JSON array of feeds. Returns an empty array `[]` if no feeds exist.

```json
[
  {
    "id": "e4b01140-5b5c-4f9e-9764-77a83d7cb45d",
    "url": "https://exampletech.com/index.xml",
    "category": "tech",
    "last_fetched_at": "2025-01-15T10:30:00Z",
    "created_at": "2025-01-14T08:00:00Z",
    "updated_at": "2025-01-15T10:30:00Z"
  },
  {
    "id": "76fa2e7d-304e-4e44-b054-945fa5cb9c20",
    "url": "https://news.ycombinator.com/rss",
    "category": "news",
    "last_fetched_at": null,
    "created_at": "2025-01-15T09:00:00Z",
    "updated_at": "2025-01-15T09:00:00Z"
  }
]
```

##### `500 Internal Server Error`
Returned when a database error occurs while fetching feeds.

```json
{
  "error": "something went wrong"
}
```

---

### Get Feed by ID

Retrieves details of a specific RSS feed using its unique UUID identifier.

* **URL:** `/api/v1/feeds/{feedID}`
* **Method:** `GET`
* **Authentication Required:** No (Public)
* **Headers:** None

---

#### URL Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `feedID` | uuid | Yes | The UUID of the feed to retrieve. |

---

#### Request Body
None.

---

#### Responses

##### `200 OK`
Returned when the feed is found.

```json
{
  "id": "e4b01140-5b5c-4f9e-9764-77a83d7cb45d",
  "url": "https://exampletech.com/index.xml",
  "category": "tech",
  "last_fetched_at": "2025-01-15T10:30:00Z",
  "created_at": "2025-01-14T08:00:00Z",
  "updated_at": "2025-01-15T10:30:00Z"
}
```

##### `400 Bad Request`
Returned when the provided `feedID` is not a valid UUID format.

```json
{
  "error": "invalid feed id"
}
```

##### `404 Not Found`
Returned when no feed matches the provided `feedID`.

```json
{
  "error": "no such feed found"
}
```

##### `500 Internal Server Error`
Returned when a database error occurs while fetching the feed.

```json
{
  "error": "something went wrong"
}
```

---

### Create Feed

Registers a new RSS feed in the system within a transaction and automatically creates a feed follow entry with a custom label for the authenticated user. The feed is scraped immediately after the response is sent.

* **URL:** `/api/v1/feeds`
* **Method:** `POST`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`
  * `Content-Type: application/json`

---

#### Request Body

| Field | Type | Required | Constraints | Description |
| :--- | :--- | :--- | :--- | :--- |
| `url` | string | Yes | Valid URL format | The RSS feed URL. |
| `name` | string | Yes | 1–45 chars, no control characters | Custom name/label for the feed follow. |

**Example Request:**

```json
{
  "url": "https://exampletech.com/index.xml",
  "name": "My Tech Blog"
}
```

---

#### Responses

##### `201 Created`
Returned when the feed is successfully created and followed.

```json
{
  "id": "c1f76e10-90fb-4d89-9a06-4b8c6e2467b2",
  "user_id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
  "feed_id": "e4b01140-5b5c-4f9e-9764-77a83d7cb45d",
  "name": "My Tech Blog",
  "user_name": "Jane Doe",
  "created_at": "2025-01-15T12:00:00Z",
  "updated_at": "2025-01-15T12:00:00Z"
}
```

##### `400 Bad Request`
Returned when the request body is malformed or input validation fails.

*Malformed JSON:*
```json
{
  "error": "invalid request body"
}
```

*Validation Errors:*
```json
{
  "error": "invalid URL"
}
```
*(or `"invalid feed name"`)*

##### `401 Unauthorized`
Returned by auth middleware when the access token is missing or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `409 Conflict`
Returned when the feed URL is already registered in the system.

```json
{
  "error": "feed already registered"
}
```

##### `500 Internal Server Error`
Returned when context lookup fails or a database transaction error occurs.

*Database/Transaction Error:*
```json
{
  "error": "something went wrong"
}
```

*Context Error:*
```json
{
  "error": "missing user id in context"
}
```

---

### Get Feed Follows

Retrieves a list of all RSS feed subscriptions (follows) for the authenticated user.

* **URL:** `/api/v1/follows`
* **Method:** `GET`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`

---

#### Request Body
None.

---

#### Responses

##### `200 OK`
Returned with a JSON array of feed follows for the authenticated user. Returns an empty array `[]` if the user has no followed feeds.

```json
[
  {
    "id": "c1f76e10-90fb-4d89-9a06-4b8c6e2467b2",
    "user_id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
    "feed_id": "e4b01140-5b5c-4f9e-9764-77a83d7cb45d",
    "name": "My Tech Blog",
    "user_name": "Jane Doe",
    "created_at": "2025-01-15T12:00:00Z",
    "updated_at": "2025-01-15T12:00:00Z"
  }
]
```

##### `401 Unauthorized`
Returned by auth middleware when the access token is missing or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `500 Internal Server Error`
Returned when context lookup fails or a database error occurs while fetching follows.

*Database Error:*
```json
{
  "error": "something went wrong"
}
```

*Context Error:*
```json
{
  "error": "missing user id in context"
}
```

---

### Follow Feed

Subscribes the authenticated user to an existing RSS feed using its feed ID.

* **URL:** `/api/v1/follows`
* **Method:** `POST`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`
  * `Content-Type: application/json`

---

#### Request Body

| Field | Type | Required | Constraints | Description |
| :--- | :--- | :--- | :--- | :--- |
| `feed_id` | uuid | Yes | Valid UUID | The UUID of the feed to follow. |
| `name` | string | Yes | 1–45 chars, no control characters | Custom label/name for the feed follow. |

**Example Request:**

```json
{
  "feed_id": "e4b01140-5b5c-4f9e-9764-77a83d7cb45d",
  "name": "My Favorite Tech Blog"
}
```

---

#### Responses

##### `201 Created`
Returned when the feed is successfully followed for the first time.

```json
{
  "id": "c1f76e10-90fb-4d89-9a06-4b8c6e2467b2",
  "user_id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
  "feed_id": "e4b01140-5b5c-4f9e-9764-77a83d7cb45d",
  "created_at": "2025-01-15T12:00:00Z",
  "updated_at": "2025-01-15T12:00:00Z",
  "name": "My Favorite Tech Blog",
  "user_name": "Jane Doe"
}
```

##### `200 OK`
Returned when the feed is already followed by the authenticated user.

```json
{
  "status": "feed already followed"
}
```

##### `400 Bad Request`
Returned when the JSON payload is malformed or input validation fails.

*Malformed JSON:*
```json
{
  "error": "invalid request body"
}
```

*Validation Errors:*
```json
{
  "error": "invalid feed name"
}
```

##### `401 Unauthorized`
Returned by auth middleware when the access token is missing or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `404 Not Found`
Returned when the specified `feed_id` does not exist in the database.

```json
{
  "error": "feed not found"
}
```

##### `500 Internal Server Error`
Returned when context lookup fails or an unexpected database error occurs.

*Database Error:*
```json
{
  "error": "something went wrong"
}
```

*Context Error:*
```json
{
  "error": "missing user id in context"
}
```

---

### Delete Feed Follow

Unfollows a feed for the authenticated user.

* **URL:** `/api/v1/follows/{feedID}`
* **Method:** `DELETE`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`

---

#### URL Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `feedID` | uuid | Yes | The UUID of the feed to unfollow. |

---

#### Request Body
None.

---

#### Responses

##### `204 No Content`
Returned when the feed follow is successfully deleted. No response body is returned.

##### `400 Bad Request`
Returned when the provided `feedID` in the URL is not a valid UUID format.

```json
{
  "error": "invalid feed id"
}
```

##### `401 Unauthorized`
Returned by auth middleware when the access token is missing or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `404 Not Found`
Returned when the authenticated user is not following the specified feed.

```json
{
  "error": "no such feed follow found"
}
```

##### `500 Internal Server Error`
Returned when context lookup fails or a database error occurs while deleting the follow.

*Database Error:*
```json
{
  "error": "something went wrong"
}
```

*Context Error:*
```json
{
  "error": "missing user id in context"
}
```

---

## Posts

### Get Posts for User

Retrieves posts from all feeds currently followed by the authenticated user, ordered by publication date descending.

* **URL:** `/api/v1/posts`
* **Method:** `GET`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`

---

#### Query Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `limit` | integer | Optional | Maximum number of posts to return (e.g., `?limit=10`). Defaults to a preset limit if omitted or invalid. |

---

#### Request Body
None.

---

#### Responses

##### `200 OK`
Returned with a JSON array of posts. Returns an empty array `[]` if no posts are found.

```json
[
  {
    "id": "2b992f9d-5a7e-4008-8e69-0df854e7c7e1",
    "title": "Why Go instead of Python for backend systems",
    "url": "https://exampletech.com/posts/why-go/",
    "description": "A deep dive into concurrency, memory management, and typing in Go.",
    "feed_id": "e4b01140-5b5c-4f9e-9764-77a83d7cb45d",
    "created_at": "2025-01-15T12:00:00Z",
    "updated_at": "2025-01-15T12:00:00Z",
    "published_at": "2025-01-15T10:00:00Z"
  }
]
```

##### `401 Unauthorized`
Returned by auth middleware when the access token is missing or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `500 Internal Server Error`
Returned when context lookup fails or a database query error occurs.

*Database Error:*
```json
{
  "error": "failed to get posts"
}
```

*Context Error:*
```json
{
  "error": "missing user id in context"
}
```

---

### Mark Post as Read

Marks a specific post as read by the authenticated user. This operation is idempotent, if the post is already marked as read, it returns a `200 OK` status confirming the existing state.

* **URL:** `/api/v1/posts/{postID}/read`
* **Method:** `POST`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`

---

#### URL Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `postID` | uuid | Yes | The UUID of the post to mark as read. |

---

#### Request Body
None.

---

#### Responses

##### `200 OK`
Returned when the post is successfully marked as read or was previously marked as read.

*Marked for the first time:*
```json
{
  "status": "post marked as read"
}
```

*Already marked as read:*
```json
{
  "status": "post already marked as read"
}
```

##### `400 Bad Request`
Returned when the `postID` URL parameter is not a valid UUID format.

```json
{
  "error": "invalid post id"
}
```

##### `401 Unauthorized`
Returned by auth middleware when the access token is missing or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `404 Not Found`
Returned when the specified `postID` does not exist in the database.

```json
{
  "error": "post not found"
}
```

##### `500 Internal Server Error`
Returned when context lookup fails or a database error occurs while marking the post as read.

*Database Error:*
```json
{
  "error": "failed to mark post read"
}
```

*Context Error:*
```json
{
  "error": "missing user id in context"
}
```

---

### Get Read Posts for User

Retrieves all posts that have been marked as read by the authenticated user, including the timestamp when each post was read.

* **URL:** `/api/v1/posts/read`
* **Method:** `GET`
* **Authentication Required:** Yes
* **Headers:**
  * `Authorization: Bearer <access_token>`

---

#### Request Body
None.

---

#### Responses

##### `200 OK`
Returned with a JSON array of read posts. Returns an empty array `[]` if no posts have been marked as read.

```json
[
  {
    "id": "2b992f9d-5a7e-4008-8e69-0df854e7c7e1",
    "title": "Why Go instead of Python for backend systems",
    "url": "https://exampletech.com/posts/why-go/",
    "description": "A deep dive into concurrency, memory management, and typing in Go.",
    "feed_id": "e4b01140-5b5c-4f9e-9764-77a83d7cb45d",
    "created_at": "2025-01-15T12:00:00Z",
    "updated_at": "2025-01-15T12:00:00Z",
    "published_at": "2025-01-15T10:00:00Z",
    "read_at": "2025-01-15T14:30:00Z"
  }
]
```

##### `401 Unauthorized`
Returned by auth middleware when the access token is missing or invalid.

```json
{
  "error": "invalid access token"
}
```

##### `500 Internal Server Error`
Returned when context lookup fails or a database query error occurs.

*Database Error:*
```json
{
  "error": "failed to get read posts for user"
}
```

*Context Error:*
```json
{
  "error": "missing user id in context"
}
```