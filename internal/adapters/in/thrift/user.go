package thrift

import (
	"context"
	"realworld-backend-go/api/thrift/gen/thriftpb"
	"realworld-backend-go/internal/domain"
)

type userService interface {
	RegisterUser(ctx context.Context, u *domain.RegisterUser) (*domain.User, error)
}

type UserServer struct {
	userService userService
}

func NewUserServer(service userService) *UserServer {
	return &UserServer{
		userService: service,
	}
}

func (u *UserServer) RegisterUser(ctx context.Context, in *thriftpb.RegisterUserRequest) (*thriftpb.UserResponse, error) {
	d := &domain.RegisterUser{
		Email:    in.GetUser().GetEmail(),
		Username: in.GetUser().GetUsername(),
		Password: in.GetUser().GetPassword(),
	}

	user, err := u.userService.RegisterUser(ctx, d)
	if err != nil {
		return nil, domainErr(err)
	}

	return &thriftpb.UserResponse{
		User: &thriftpb.UserResponseInner{
			Email:    user.Email,
			Token:    user.Token,
			Username: user.Username,
			Bio:      user.Bio,
			Image:    user.Image,
		},
	}, nil
}
