# Tài liệu API của HAB

Tài liệu này cung cấp một cái nhìn tổng quan chi tiết về các điểm cuối (endpoint) API cho ứng dụng HAB (Homework Auto-grader Backend).

## Mục lục
1.  [Xác thực](#xác-thực)
2.  [Các khái niệm chung](#các-khái-niệm-chung)
3.  [Các Endpoint API](#các-endpoint-api)
    *   [Auth](#auth)
    *   [Bài toán (Problems)](#bài-toán-problems)
    *   [Bài nộp (Submissions)](#bài-nộp-submissions)
    *   [Kiểm tra hệ thống (Health Check)](#kiểm-tra-hệ-thống-health-check)

## Xác thực

API sử dụng HTTP-only cookie để quản lý phiên làm việc.
-   `access_token`: Một JWT (JSON Web Token) có thời hạn ngắn (1 giờ) dùng để xác thực các yêu cầu.
-   `refresh_token`: Một JWT có thời hạn dài (14 ngày) dùng để lấy `access_token` mới.

Hầu hết các endpoint yêu cầu xác thực. Phía client nên xử lý cookie `access_token` và `refresh_token` một cách tự động.

## Các khái niệm chung

-   **`language_id`**: Mã số định danh cho các ngôn ngữ lập trình.
    -   `1`: Python
    -   `2`: Go

## Các Endpoint API

---

### Auth

Đường dẫn cơ sở: `/auth`

#### Đăng ký người dùng mới

-   **Endpoint:** `POST /auth/register`
-   **Mô tả:** Tạo một tài khoản người dùng mới.
-   **Xác thực:** Không yêu cầu.
-   **Request Body:**

    ```json
    {
      "username": "newuser",
      "email": "user@example.com",
      "password": "password123"
    }
    ```

-   **Phản hồi (Responses):**
    -   **`201 Created`**:
        ```json
        {
          "success": true
        }
        ```
    -   **`400 Bad Request`**: Nếu dữ liệu không hợp lệ (ví dụ: mật khẩu quá ngắn, email không đúng định dạng).
        ```json
        {
          "error": "password must be at least 8 characters long"
        }
        ```
    -   **`409 Conflict`**: Nếu tên người dùng hoặc email đã tồn tại.
        ```json
        {
          "error": "Username or email already exists"
        }
        ```

#### Đăng nhập

-   **Endpoint:** `POST /auth/login`
-   **Mô tả:** Xác thực người dùng và thiết lập cookie `access_token` và `refresh_token`.
-   **Xác thực:** Không yêu cầu.
-   **Request Body:**

    ```json
    {
      "email": "user@example.com",
      "password": "password123"
    }
    ```

-   **Phản hồi:**
    -   **`200 OK`**: Thiết lập cookie trong header của phản hồi.
        ```json
        {
          "success": true
        }
        ```
    -   **`401 Unauthorized`**: Nếu thông tin đăng nhập không hợp lệ.
        ```json
        {
          "success": false,
          "error": "Invalid credentials"
        }
        ```

#### Đăng xuất

-   **Endpoint:** `POST /auth/logout`
-   **Mô tả:** Xóa các cookie xác thực.
-   **Xác thực:** Không yêu cầu, nhưng dành cho người dùng đã đăng nhập.
-   **Phản hồi:**
    -   **`200 OK`**:
        ```json
        {
          "message": "Logged out successfully"
        }
        ```

#### Kiểm tra trạng thái xác thực

-   **Endpoint:** `GET /auth/verify`
-   **Mô tả:** Kiểm tra xem người dùng đã được xác thực hay chưa. Nếu `access_token` đã hết hạn nhưng `refresh_token` vẫn hợp lệ, hệ thống sẽ cấp một `access_token` mới.
-   **Xác thực:** Không yêu cầu.
-   **Phản hồi:**
    -   **`200 OK`**:
        ```json
        {
          "is_authenticated": true,
          "user_id": 123
        }
        ```
    -   **`401 Unauthorized`**: Nếu cả hai token đều thiếu hoặc không hợp lệ.
        ```json
        {
          "is_authenticated": false,
          "error": "Authorization required"
        }
        ```

---

### Bài toán (Problems)

Đường dẫn cơ sở: `/problems`

#### Lấy tất cả bài toán

-   **Endpoint:** `GET /problems`
-   **Mô tả:** Lấy danh sách tất cả các bài toán có sẵn. Nếu người dùng đã xác thực, kết quả sẽ cho biết bài toán nào đã được giải.
-   **Xác thực:** Tùy chọn.
-   **Phản hồi:**
    -   **`200 OK`**:
        ```json
        {
          "problems": [
            {
              "id": 1,
              "title": "Two Sum",
              "difficulty": "Easy",
              "is_solved": true
            },
            {
              "id": 2,
              "title": "Reverse String",
              "difficulty": "Easy",
              "is_solved": false
            }
          ]
        }
        ```
    -   **`500 Internal Server Error`**:
        ```json
        {
          "error": "Failed to retrieve problems"
        }
        ```

#### Lấy bài toán theo ID

-   **Endpoint:** `GET /problems/:id`
-   **Mô tả:** Lấy thông tin chi tiết cho một bài toán cụ thể, bao gồm cả code khởi tạo cho các ngôn ngữ được hỗ trợ.
-   **Xác thực:** Tùy chọn.
-   **Phản hồi:**
    -   **`200 OK`**:
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
    -   **`404 Not Found`**:
        ```json
        {
          "error": "Problem not found"
        }
        ```

---

### Bài nộp (Submissions)

Đường dẫn cơ sở: `/submissions`

#### Tạo một bài nộp mới

-   **Endpoint:** `POST /submissions`
-   **Mô tả:** Nộp code cho một bài toán cụ thể để chấm điểm.
-   **Xác thực:** Yêu cầu.
-   **Request Body:**

    ```json
    {
      "problem_id": 1,
      "language_id": 1,
      "source_code": "def twoSum(nums, target):\n  return [0, 1]"
    }
    ```

-   **Phản hồi:**
    -   **`202 Accepted`**: Bài nộp đã được đưa vào hàng đợi để xử lý.
        ```json
        {
          "message": "Submission queued for processing",
          "submission_id": 42
        }
        ```
    -   **`400 Bad Request`**: Nếu request body không hợp lệ.
        ```json
        {
          "error": "source code cannot be empty"
        }
        ```
    -   **`401 Unauthorized`**:
        ```json
        {
          "error": "User not authenticated"
        }
        ```

#### Lấy chi tiết bài nộp theo ID

-   **Endpoint:** `GET /submissions/:id`
-   **Mô tả:** Lấy trạng thái và chi tiết của một bài nộp cụ thể.
-   **Xác thực:** Yêu cầu.
-   **Phản hồi:**
    -   **`200 OK` (Chấp nhận - Accepted)**:
        ```json
        {
          "status": "ACCEPTED",
          "source_code": "..."
        }
        ```
    -   **`200 OK` (Đáp án sai - Wrong Answer)**:
        ```json
        {
          "status": "WRONG_ANSWER",
          "source_code": "...",
          "wrong_testcase": "Input: [3, 3], Target: 6",
          "expected_output": "[0, 1]",
          "program_output": "[1, 0]"
        }
        ```
    -   **`200 OK` (Lỗi biên dịch - Compilation Error)**:
        ```json
        {
          "status": "COMPILATION_ERROR",
          "source_code": "...",
          "program_output": "Syntax error on line 5..."
        }
        ```
    -   **`404 Not Found`**: Nếu bài nộp không tồn tại hoặc người dùng không có quyền truy cập.
        ```json
        {
          "error": "Submission not found or access denied"
        }
        ```

#### Lấy các bài nộp của người dùng cho một bài toán

-   **Endpoint:** `GET /submissions?problem_id=<problem_id>`
-   **Mô tả:** Lấy danh sách tất cả các bài nộp của người dùng hiện tại cho một bài toán cụ thể.
-   **Xác thực:** Yêu cầu.
-   **Tham số truy vấn (Query Parameters):**
    -   `problem_id` (bắt buộc): ID của bài toán.
-   **Phản hồi:**
    -   **`200 OK`**:
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
    -   **`400 Bad Request`**: Nếu `problem_id` bị thiếu hoặc không hợp lệ.
        ```json
        {
          "error": "problem_id query parameter is required"
        }
        ```

---

### Kiểm tra hệ thống (Health Check)

#### Kiểm tra "sức khỏe" của máy chủ

-   **Endpoint:** `GET /health`
-   **Mô tả:** Một endpoint đơn giản để xác minh rằng máy chủ đang hoạt động.
-   **Xác thực:** Không yêu cầu.
-   **Phản hồi:**
    -   **`200 OK`**:
        ```json
        {
          "status": "healthy"
        }
        ```