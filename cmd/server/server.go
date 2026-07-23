package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	pb "realworld-backend-go/api/proto/gen/pb"
	igrpc "realworld-backend-go/internal/adapters/in/grpc"
	ithrift "realworld-backend-go/internal/adapters/in/thrift"
	"realworld-backend-go/internal/adapters/in/webserver"
	"realworld-backend-go/internal/adapters/out/db"
	"realworld-backend-go/internal/domain"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
)

func main() {
	// load configs
	envFile := flag.String("env", ".env", "path to env file")
	flag.Parse()
	if _, err := os.Stat(*envFile); err == nil {
		if err := godotenv.Load(*envFile); err != nil {
			log.Fatal(err)
		}
	}

	// Setup all dependencies
	database, err := db.New(&db.DBConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
	})
	if err != nil {
		log.Fatal(err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	userController := domain.New(database, jwtSecret)
	profileController := domain.NewProfileController(database)
	articleController := domain.NewArticleController(database, database, database)
	tagController := domain.NewTagController(database)
	commentController := domain.NewCommentController(database, database, database)
	handlers := webserver.NewHandler(userController, profileController, articleController, tagController, commentController)

	port := os.Getenv("SERVER_PORT")

	log.Printf("starting server on port %s...\n", port)

	s, err := webserver.NewServer(port, handlers, jwtSecret)
	if err != nil {
		log.Fatal(err)
	}

	// gRPC server
	authMethods := map[string]igrpc.AuthRequirement{
		pb.UserService_GetUser_FullMethodName:              igrpc.MandatoryAuth,
		pb.UserService_UpdateUser_FullMethodName:           igrpc.MandatoryAuth,
		pb.ProfileService_GetProfile_FullMethodName:        igrpc.OptionalAuth,
		pb.ProfileService_FollowUser_FullMethodName:        igrpc.MandatoryAuth,
		pb.ProfileService_UnfollowUser_FullMethodName:      igrpc.MandatoryAuth,
		pb.ArticleService_CreateArticle_FullMethodName:     igrpc.MandatoryAuth,
		pb.ArticleService_GetArticleBySlug_FullMethodName:  igrpc.OptionalAuth,
		pb.ArticleService_UpdateArticle_FullMethodName:     igrpc.MandatoryAuth,
		pb.ArticleService_FavoriteArticle_FullMethodName:   igrpc.MandatoryAuth,
		pb.ArticleService_UnfavoriteArticle_FullMethodName: igrpc.MandatoryAuth,
		pb.ArticleService_DeleteArticle_FullMethodName:     igrpc.MandatoryAuth,
		pb.ArticleService_ListArticles_FullMethodName:      igrpc.OptionalAuth,
		pb.ArticleService_FeedArticles_FullMethodName:      igrpc.MandatoryAuth,
		pb.CommentService_CreateComment_FullMethodName:     igrpc.MandatoryAuth,
		pb.CommentService_GetComments_FullMethodName:       igrpc.OptionalAuth,
		pb.CommentService_DeleteComment_FullMethodName:     igrpc.MandatoryAuth,
	}
	streamAuthMethods := map[string]igrpc.AuthRequirement{
		pb.ArticleService_LiveArticleFeed_FullMethodName: igrpc.MandatoryAuth,
		pb.CommentService_LiveCommentFeed_FullMethodName: igrpc.OptionalAuth,
	}

	// creds for mTLS
	tlsConfig := createTlsConfig()
	creds := setupTLSCreds(tlsConfig)

	grpcServer := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(igrpc.AuthInterceptor(jwtSecret, authMethods)),
		grpc.StreamInterceptor(igrpc.StreamAuthInterceptor(jwtSecret, streamAuthMethods)),
	)
	userGrpcServer := igrpc.NewUserServer(userController)
	tagGrpcServer := igrpc.NewTagServer(tagController)
	profileGrpcServer := igrpc.NewProfileServer(profileController)
	commentGrpcServer := igrpc.NewCommentServer(commentController)
	articleGrpcServer := igrpc.NewArticleServer(articleController)
	healthServer := health.NewServer()
	igrpcServer := igrpc.NewGrpcServer(grpcServer, healthServer, userGrpcServer, tagGrpcServer, profileGrpcServer, commentGrpcServer, articleGrpcServer)

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		log.Fatal("GRPC_PORT environment variable is required")
	}
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	go func() {
		log.Printf("starting gRPC server on port %s...", grpcPort)
		if err := igrpcServer.Server.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// Thrift http server!
	thriftPort := os.Getenv("THRIFT_HTTP_PORT")
	thriftAddr := fmt.Sprintf("localhost:%s", thriftPort)

	userThriftService := ithrift.NewUserServer(userController)
	thriftServer, err := ithrift.NewThriftServer(thriftAddr, userThriftService, tlsConfig)
	if err != nil {
		log.Fatalf("failed to create Thrift http server: %v", err)
	}
	go func() {
		log.Printf("starting Thrift server on port %s...", thriftPort)
		if err = thriftServer.Server.Serve(); err != nil {
			log.Fatalf("Thrift server failed: %v", err)
		}
	}()

	s.Start()
}

func setupTLSCreds(cfg *tls.Config) credentials.TransportCredentials {
	creds := credentials.NewTLS(cfg)
	return creds
}

func createTlsConfig() *tls.Config {
	certPEM, err := os.ReadFile(os.Getenv("GRPC_TLS_CERT"))
	if err != nil {
		log.Fatalf("error loading cert PEM: %v", err)
	}

	keyPEM, err := os.ReadFile(os.Getenv("GRPC_TLS_KEY"))
	if err != nil {
		log.Fatalf("error loading key PEM: %v", err)
	}

	caPEM, err := os.ReadFile(os.Getenv("GRPC_TLS_CA"))
	if err != nil {
		log.Fatalf("error loading CA PEM: %v", err)
	}

	serverCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		log.Fatalf("could not load x509 key pair: %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPEM) {
		log.Fatalf("Failed to append CA certificate to pool")

	}
	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	}
	return &tlsConfig
}
