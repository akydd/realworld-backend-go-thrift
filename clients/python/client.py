import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "gen"))

from thrift.transport import TSocket, TTransport
from thrift.protocol import TBinaryProtocol

from thriftpb import UserService
from thriftpb.ttypes import (
    RegisterUserRequest,
    RegisterUserRequestInner,
    ValidationError,
)


def main():
    socket = TSocket.TSocket("localhost", 8109)
    transport = TTransport.TBufferedTransport(socket)
    protocol = TBinaryProtocol.TBinaryProtocol(transport)
    client = UserService.Client(protocol)

    transport.open()
    try:
        resp = client.registerUser(
            RegisterUserRequest(
                user=RegisterUserRequestInner(
                    username="Test",
                    email="test@test.com",
                    password="password",
                )
            )
        )
        print("new user:", resp.user)
    except ValidationError as e:
        print("validation error:", e.errors)
    finally:
        transport.close()


if __name__ == "__main__":
    main()
