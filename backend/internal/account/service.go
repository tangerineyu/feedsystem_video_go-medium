package account

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/auth"
	"log"
	"time"

	rediscache "feedsystem_video_go/internal/middleware/redis"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AccountService struct {
	accountRepository *AccountRepository
	cache             *rediscache.Client
}

var (
	ErrUsernameTaken       = errors.New("username already exists")
	ErrNewUsernameRequired = errors.New("new_username is required")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
	ErrRefreshTokenRevoked = errors.New("refresh token has been revoked")
)

func NewAccountService(accountRepository *AccountRepository, cache *rediscache.Client) *AccountService {
	return &AccountService{accountRepository: accountRepository, cache: cache}
}

func (as *AccountService) CreateAccount(ctx context.Context, account *Account) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(account.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	account.Password = string(passwordHash)
	if err := as.accountRepository.CreateAccount(ctx, account); err != nil {
		return err
	}
	return nil
}

func (as *AccountService) Rename(ctx context.Context, accountID uint, newUsername string) (*TokenPairResponse, error) {
	if newUsername == "" {
		return nil, ErrNewUsernameRequired
	}

	resp, refreshHash, err := as.issueTokenPair(accountID, newUsername)
	if err != nil {
		return nil, err
	}

	if err := as.accountRepository.RenameWithToken(ctx, accountID, newUsername, resp.AccessToken, refreshHash); err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return nil, ErrUsernameTaken
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	as.writeTokenCache(ctx, accountID, resp.AccessToken, refreshHash)
	return resp, nil
}

func (as *AccountService) Refresh(ctx context.Context, refreshToken string) (*TokenPairResponse, error) {
	claims, err := auth.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}
	if err := as.validateRefreshToken(ctx, claims.AccountID, refreshToken); err != nil {
		return nil, err
	}

	accountInfo, err := as.accountRepository.FindByID(ctx, claims.AccountID)
	if err != nil {
		return nil, err
	}
	resp, newRefreshHash, err := as.issueTokenPair(accountInfo.ID, accountInfo.Username)
	if err != nil {
		return nil, err
	}
	if err := as.accountRepository.UpdateTokens(ctx, accountInfo.ID, resp.AccessToken, newRefreshHash); err != nil {
		return nil, err
	}
	as.writeTokenCache(ctx, accountInfo.ID, resp.AccessToken, newRefreshHash)
	return resp, nil
}
func (as *AccountService) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	account, err := as.FindByUsername(ctx, username)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(oldPassword)); err != nil {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := as.accountRepository.ChangePassword(ctx, account.ID, string(passwordHash)); err != nil {
		return err
	}
	if err := as.Logout(ctx, account.ID); err != nil {
		return err
	}
	return nil
}

func (as *AccountService) FindByID(ctx context.Context, id uint) (*Account, error) {
	account, err := as.accountRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (as *AccountService) FindByUsername(ctx context.Context, username string) (*Account, error) {
	account, err := as.accountRepository.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (as *AccountService) issueTokenPair(accountID uint, username string) (*TokenPairResponse, string, error) {
	accessToken, refreshToken, err := auth.GenerateTokenPair(accountID, username)
	if err != nil {
		return nil, "", err
	}
	resp := &TokenPairResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
	return resp, HashRefreshToken(refreshToken), nil
}

func (as *AccountService) writeTokenCache(ctx context.Context, accountID uint, accessToken, refreshHash string) {
	if as.cache == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := as.cache.SetBytes(cacheCtx, SessionCacheKey(accountID), []byte(accessToken), auth.AccessTokenTTL()); err != nil {
		log.Printf("failed to set access token cache: %v", err)
	}
	if err := as.cache.SetBytes(cacheCtx, RefreshHashKey(accountID), []byte(refreshHash), auth.RefreshTokenTTL()); err != nil {
		log.Printf("failed to set refresh token cache: %v", err)
	}
}

func (as *AccountService) clearTokenCache(ctx context.Context, accountID uint) {
	if as.cache == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := as.cache.Del(cacheCtx, SessionCacheKey(accountID)); err != nil {
		log.Printf("failed to del access token cache: %v", err)
	}
	if err := as.cache.Del(cacheCtx, RefreshHashKey(accountID)); err != nil {
		log.Printf("failed to del refresh token cache: %v", err)
	}
}

// validateRefreshToken checks a raw refresh token against the stored hash.
func (as *AccountService) validateRefreshToken(ctx context.Context, accountID uint, refreshToken string) error {
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		b, err := as.cache.GetBytes(cacheCtx, RefreshHashKey(accountID))
		if err == nil {
			if !RefreshTokenMatchesHash(refreshToken, string(b)) {
				return ErrRefreshTokenRevoked
			}
			return nil
		}
	}

	accountInfo, err := as.accountRepository.FindByID(ctx, accountID)
	if err != nil || !RefreshTokenMatchesHash(refreshToken, accountInfo.RefreshTokenHash) {
		return ErrRefreshTokenRevoked
	}

	refreshHash := HashRefreshToken(refreshToken)
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		if err := as.cache.SetBytes(cacheCtx, RefreshHashKey(accountID), []byte(refreshHash), auth.RefreshTokenTTL()); err != nil {
			log.Printf("failed to set refresh token cache: %v", err)
		}
	}
	return nil
}

func (as *AccountService) Login(ctx context.Context, username, password string) (*TokenPairResponse, error) {
	account, err := as.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)); err != nil {
		return nil, err
	}
	resp, refreshHash, err := as.issueTokenPair(account.ID, account.Username)
	if err != nil {
		return nil, err
	}
	if err := as.accountRepository.UpdateTokens(ctx, account.ID, resp.AccessToken, refreshHash); err != nil {
		return nil, err
	}
	as.writeTokenCache(ctx, account.ID, resp.AccessToken, refreshHash)
	return resp, nil
}

func (as *AccountService) Logout(ctx context.Context, accountID uint) error {
	account, err := as.FindByID(ctx, accountID)
	if err != nil {
		return err
	}
	if account.Token == "" && account.RefreshTokenHash == "" {
		return nil
	}
	as.clearTokenCache(ctx, accountID)
	return as.accountRepository.Logout(ctx, account.ID)
}
