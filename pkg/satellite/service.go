package satellite

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/satellite/satelliteauth"
)

// Service is handling accounts related logic
type Service struct {
	Signer

	store DB
}

// NewService returns new instance of Service
func NewService(signer Signer, store DB) (*Service, error) {
	if signer == nil {
		return nil, errs.New("signer can't be nil")
	}

	if store == nil {
		return nil, errs.New("store can't be nil")
	}

	return &Service{Signer: signer, store: store}, nil
}

// Register gets password hash value and creates new user
func (s *Service) Register(ctx context.Context, user *User) (*User, error) {
	passwordHash := sha256.Sum256(user.PasswordHash)
	user.PasswordHash = passwordHash[:]

	newUser, err := s.store.Users().Insert(ctx, user)
	if err != nil {
		return nil, err
	}

	return newUser, nil
}

// Login authenticates user by credentials and returns auth token
func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	passwordHash := sha256.Sum256([]byte(password))

	user, err := s.store.Users().GetByCredentials(ctx, passwordHash[:], email)
	if err != nil {
		return "", err
	}

	//TODO: move expiration time to constants
	claims := satelliteauth.Claims{
		ID:         user.ID,
		Expiration: time.Now().Add(time.Minute * 15),
	}

	token, err := s.createToken(&claims)
	if err != nil {
		return "", err
	}

	return token, nil
}

// GetUser returns user by id
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	token, ok := auth.GetAPIKey(ctx)
	if !ok {
		return nil, errs.New("no api key was provided")
	}

	claims, err := s.authenticate(string(token))
	if err != nil {
		return nil, err
	}

	err = s.authorize(ctx, claims)
	if err != nil {
		return nil, err
	}

	user, err := s.store.Users().Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) createToken(claims *satelliteauth.Claims) (string, error) {
	json, err := claims.JSON()
	if err != nil {
		return "", err
	}

	token := satelliteauth.Token{Payload: json}
	err = signToken(&token, s.Signer)
	if err != nil {
		return "", err
	}

	return token.String(), nil
}

func (s *Service) authenticate(tokenS string) (*satelliteauth.Claims, error) {
	token, err := satelliteauth.FromBase64URLString(tokenS)
	if err != nil {
		return nil, err
	}

	signature := token.Signature

	err = signToken(&token, s.Signer)
	if err != nil {
		return nil, err
	}

	if subtle.ConstantTimeCompare(signature, token.Signature) != 1 {
		return nil, errs.New("incorrect signature")
	}

	claims, err := satelliteauth.FromJSON(token.Payload)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func (s *Service) authorize(ctx context.Context, claims *satelliteauth.Claims) error {
	if !claims.Expiration.IsZero() && claims.Expiration.Before(time.Now()) {
		return errs.New("token is outdated")
	}

	_, err := s.store.Users().Get(ctx, claims.ID)
	if err != nil {
		return errs.New("authorization failed. no user with id: %s", claims.ID.String())
	}

	return nil
}
