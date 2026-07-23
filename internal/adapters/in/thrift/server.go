package thrift

import (
	"crypto/tls"
	"realworld-backend-go/api/thrift/gen/thriftpb"

	"github.com/apache/thrift/lib/go/thrift"
)

type ThriftServer struct {
	Server *thrift.TSimpleServer
}

func NewThriftServer(addr string, userService thriftpb.UserService, tlsCfg *tls.Config) (*ThriftServer, error) {
	processor := thriftpb.NewUserServiceProcessor(userService)

	var transport thrift.TServerTransport
	var err error

	if tlsCfg != nil {
		transport, err = thrift.NewTSSLServerSocket(addr, tlsCfg)
	} else {
		transport, err = thrift.NewTServerSocket(addr)
	}
	if err != nil {
		return nil, err
	}

	protoFactory := thrift.NewTBinaryProtocolFactoryConf(nil)
	transportFactory := thrift.NewTBufferedTransportFactory(8192)

	server := thrift.NewTSimpleServer4(processor, transport, transportFactory, protoFactory)

	return &ThriftServer{
		Server: server,
	}, nil
}
