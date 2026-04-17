package domain

import "errors"

var (
	ErrEmailTaken          = errors.New("user with this email already exists")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrAccountLocked       = errors.New("account is temporarily locked due to too many failed attempts")
	ErrTwoFARequired       = errors.New("two-factor authentication code required")
	ErrInvalidTOTP         = errors.New("invalid or expired two-factor authentication code")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
	ErrTOTPAlreadyEnabled  = errors.New("two-factor authentication is already enabled")
	ErrTOTPNotEnabled      = errors.New("two-factor authentication is not enabled")

	ErrTransferNotFound     = errors.New("transfer not found")
	ErrTransferExpired      = errors.New("transfer has expired")
	ErrTransferRevoked      = errors.New("transfer has been revoked")
	ErrDownloadLimitReached = errors.New("download limit reached")
	ErrAuthRequired         = errors.New("authentication required to access this transfer")
	ErrEmailNotAllowed      = errors.New("your email is not in the allowed list")

	ErrFileTooLarge  = errors.New("file exceeds maximum allowed size")
	ErrInvalidPolicy = errors.New("invalid access policy: expiry must be in the future")
	ErrNoFile        = errors.New("no file provided")

	ErrTransferNotActive = errors.New("transfer is not in ACTIVE status")
	ErrNotOwner          = errors.New("you are not the owner of this transfer")

	ErrInvalidSettings = errors.New("invalid settings: max file size and retention must be positive")
)
