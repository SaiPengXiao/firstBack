package model

// User is returned to clients after login/register.
type User struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
}

// Permission codes (resource:action). Stored in the permissions table and
// referenced by RequirePermission middleware / route registration.
const (
	PermMenuRead       = "menu:read"
	PermCategoryCreate = "menu:category:create"
	PermCategoryUpdate = "menu:category:update"
	PermCategoryDelete = "menu:category:delete"
	PermItemCreate     = "menu:item:create"
	PermItemUpdate     = "menu:item:update"
	PermItemDelete     = "menu:item:delete"
	PermOrderRead      = "order:read"
)

// LoginRequest matches front/react LoginFormValues.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest matches front/react RegisterFormValues (confirmPassword validated client-side).
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=2"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// AuthResponse is the unified success payload for auth endpoints.
type AuthResponse struct {
	User  User   `json:"user"`
	Token string `json:"token"`
}

// WechatLoginRequest body for POST /api/auth/wechat-login.
type WechatLoginRequest struct {
	Code string `json:"code" binding:"required"`
}

// WechatLoginResponse is the success payload for WeChat login.
type WechatLoginResponse struct {
	Token     string         `json:"token"`
	ExpiresIn int            `json:"expiresIn"`
	User      WechatUserInfo `json:"user"`
}

// WechatUserInfo is the public user profile returned after WeChat login.
type WechatUserInfo struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"displayName"`
	Nickname    *string `json:"nickname"`
	AvatarURL   *string `json:"avatarUrl"`
	Phone       *string `json:"phone"`
}

// ErrorResponse is used for API errors.
type ErrorResponse struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}