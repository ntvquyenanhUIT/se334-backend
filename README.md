# HAB — Homework Auto‑Grader Backend

A backend service for managing coding problems and evaluating user submissions. This document provides an overview and API reference for the current implementation.

## Overview

HAB exposes RESTful endpoints for:
- User authentication with HTTP-only cookies (JWT access and refresh tokens)
- Browsing coding problems and their starter code
- Submitting solutions and retrieving submission results
- Basic server health check

Most endpoints require authentication.

## Key Capabilities

- HTTP-only cookie authentication:
  - access_token (short-lived, 1 hour)
  - refresh_token (long-lived, 14 days)
- Problem browsing with “is_solved” flag for authenticated users
- Problem details include starter code per supported language
- Submission lifecycle with statuses:
  - ACCEPTED
  - WRONG_ANSWER
  - COMPILATION_ERROR
- Asynchronous submission handling (202 Accepted when queued)

## Common Concepts

- language_id:
  - 1: Python
  - 2: Go

## Authentication

- Cookies:
  - access_token: short-lived JWT for request auth
  - refresh_token: long-lived JWT to mint a new access_token
- Client should handle cookies automatically.
- If access_token expires but refresh_token is valid, the system issues a new access_token.

## API Reference

Base paths:
- Auth: `/auth`
- Problems: `/problems`
- Submissions: `/submissions`
- Health: `/health`

### Auth

#### Register
- POST `/auth/register`
- Create a new user.
- Auth: Not required
- Body:
```json
{
  "username": "newuser",
  "email": "user@example.com",
  "password": "password123"
}
```
- Responses:
  - 201:
    ```json
    { "success": true }
    ```
  - 400:
    ```json
    { "error": "password must be at least 8 characters long" }
    ```
  - 409:
    ```json
    { "error": "Username or email already exists" }
    ```

#### Login
- POST `/auth/login`
- Sets access_token and refresh_token cookies.
- Auth: Not required
- Body:
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```
- Responses:
  - 200:
    ```json
    { "success": true }
    ```
  - 401:
    ```json
    { "success": false, "error": "Invalid credentials" }
    ```

#### Logout
- POST `/auth/logout`
- Clears auth cookies.
- Auth: Not required (intended for logged-in users)
- Response:
  - 200:
    ```json
    { "message": "Logged out successfully" }
    ```

#### Verify
- GET `/auth/verify`
- Checks authentication; may refresh access_token using a valid refresh_token.
- Auth: Not required
- Responses:
  - 200:
    ```json
    { "is_authenticated": true, "user_id": 123 }
    ```
  - 401:
    ```json
    { "is_authenticated": false, "error": "Authorization required" }
    ```

### Problems

#### List Problems
- GET `/problems`
- Returns all problems. If authenticated, includes is_solved per problem.
- Auth: Optional
- Responses:
  - 200:
    ```json
    {
      "problems": [
        { "id": 1, "title": "Two Sum", "difficulty": "Easy", "is_solved": true },
        { "id": 2, "title": "Reverse String", "difficulty": "Easy", "is_solved": false }
      ]
    }
    ```
  - 500:
    ```json
    { "error": "Failed to retrieve problems" }
    ```

#### Get Problem by ID
- GET `/problems/:id`
- Returns detailed problem info and starter code by language.
- Auth: Optional
- Responses:
  - 200:
    ```json
    {
      "id": 1,
      "title": "Two Sum",
      "description": "Given an array of integers...",
      "difficulty": "Easy",
      "sample_input": "nums = [2,7,11,15], target = 9",
      "sample_output": "[0,1]",
      "starter_code": {
        "1": "def twoSum(nums, target):\n  # Your code here",
        "2": "package main\n\nfunc twoSum(nums []int, target int) []int {\n  // Your code here\n}"
      },
      "is_solved": true
    }
    ```
  - 404:
    ```json
    { "error": "Problem not found" }
    ```

### Submissions

#### Create Submission
- POST `/submissions`
- Submit code for a problem.
- Auth: Required
- Body:
```json
{
  "problem_id": 1,
  "language_id": 1,
  "source_code": "def twoSum(nums, target):\n  return [0, 1]"
}
```
- Responses:
  - 202:
    ```json
    { "message": "Submission queued for processing", "submission_id": 42 }
    ```
  - 400:
    ```json
    { "error": "source code cannot be empty" }
    ```
  - 401:
    ```json
    { "error": "User not authenticated" }
    ```

#### Get Submission by ID
- GET `/submissions/:id`
- Returns submission status and details.
- Auth: Required
- Responses:
  - 200 (ACCEPTED):
    ```json
    { "status": "ACCEPTED", "source_code": "..." }
    ```
  - 200 (WRONG_ANSWER):
    ```json
    {
      "status": "WRONG_ANSWER",
      "source_code": "...",
      "wrong_testcase": "Input: [3, 3], Target: 6",
      "expected_output": "[0, 1]",
      "program_output": "[1, 0]"
    }
    ```
  - 200 (COMPILATION_ERROR):
    ```json
    {
      "status": "COMPILATION_ERROR",
      "source_code": "...",
      "program_output": "Syntax error on line 5..."
    }
    ```
  - 404:
    ```json
    { "error": "Submission not found or access denied" }
    ```

#### List User Submissions for a Problem
- GET `/submissions?problem_id=<problem_id>`
- Returns current user’s submissions for a problem.
- Auth: Required
- Query:
  - problem_id (required)
- Responses:
  - 200:
    ```json
    {
      "submissions": [
        {
          "id": 42,
          "language_id": 1,
          "status": "ACCEPTED",
          "submitted_at": "2023-10-27T10:00:00Z",
          "submitted_time": "27/10/2023 10:00AM",
          "language_name": "Python"
        }
      ],
      "count": 1
    }
    ```
  - 400:
    ```json
    { "error": "problem_id query parameter is required" }
    ```

### Health

#### Health Check
- GET `/health`
- Verifies server is up.
- Auth: Not required
- Response:
  - 200:
    ```json
    { "status": "healthy" }
    ```

## Error Format

Errors return a JSON body with an error message:
```json
{ "error": "Message here" }
```

## Notes

- Keep cookies enabled on the client to allow automatic token handling.
- Supported languages are identified via language_id as listed above.
- Submission processing is asynchronous; poll submission by