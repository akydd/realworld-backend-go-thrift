namespace go thriftpb
namespace py thriftpb

struct RegisterUserRequestInner {
    1: string username,
    2: string email,
    3: string password
}

struct RegisterUserRequest {
    1: RegisterUserRequestInner user
}

struct UserResponseInner {
    1: string email,
    2: string token,
    3: string username,
    4: optional string bio,
    5: optional string image
}

struct UserResponse {
    1: UserResponseInner user
}

exception ValidationError {
    1: map<string, list<string>> errors
}


service UserService {
    UserResponse registerUser(1: RegisterUserRequest request) throws (1: ValidationError e)
}
