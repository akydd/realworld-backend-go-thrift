import os
import sys
import ssl

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "gen"))

CERTS = os.path.join(os.path.dirname(__file__), "..", "..", "certs")

from thrift.transport import TSSLSocket, TTransport
from thrift.protocol import TBinaryProtocol

from thriftpb import UserService
from thriftpb.ttypes import (
    RegisterUserRequest,
    RegisterUserRequestInner,
    ValidationError,
)


def main():
    ctx = ssl.create_default_context(
        ssl.Purpose.SERVER_AUTH, cafile=os.path.join(CERTS, "ca.crt")
    )
    ctx.load_cert_chain(
        certfile=os.path.join(CERTS, "client.crt"),
        keyfile=os.path.join(CERTS, "client.key"),
    )
    ctx.minimum_version = ssl.TLSVersion.TLSv1_3

    socket = TSSLSocket.TSSLSocket(
        "localhost", 8100, ssl_context=ctx, server_hostname="localhost"
    )
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
