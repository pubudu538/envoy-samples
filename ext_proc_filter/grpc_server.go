package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	v3alpha "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3alpha"
	pb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3alpha"
)

var (
	grpcport = flag.String("grpcport", ":8080", "grpcport")
	hs       *health.Server
)

const ()

type server struct{}

type healthServer struct{}

func (s *healthServer) Check(ctx context.Context, in *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	log.Printf("Handling grpc Check request + %s", in.String())
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (s *healthServer) Watch(in *healthpb.HealthCheckRequest, srv healthpb.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}

func (s *server) Process(srv pb.ExternalProcessor_ProcessServer) error {

	log.Println("Got stream:  -->  ")
	ctx := srv.Context()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		req, err := srv.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "cannot receive stream request: %v", err)
		}

		resp := &pb.ProcessingResponse{}
		switch v := req.Request.(type) {
		case *pb.ProcessingRequest_RequestHeaders:

			log.Printf("pb.ProcessingRequest_RequestHeaders %v \n", v)
			r := req.Request
			h := r.(*pb.ProcessingRequest_RequestHeaders)

			// Printing Headers
			for _, n := range h.RequestHeaders.Headers.Headers {
				log.Printf("Header %s %s", n.Key, n.Value)
			}

			isPOST := false
			for _, n := range h.RequestHeaders.Headers.Headers {
				if n.Key == ":method" && n.Value == "POST" {
					isPOST = true
					break
				}
			}

			rhq := &pb.HeadersResponse{
				Response: &pb.CommonResponse{
					HeaderMutation: &pb.HeaderMutation{
						RemoveHeaders: []string{"authorization"},
					},
				},
			}

			if isPOST {
				resp = &pb.ProcessingResponse{
					Response: &pb.ProcessingResponse_RequestHeaders{
						RequestHeaders: rhq,
					},
					// Override Processing Mode
					ModeOverride: &v3alpha.ProcessingMode{
						RequestBodyMode:    v3alpha.ProcessingMode_BUFFERED,
						ResponseHeaderMode: v3alpha.ProcessingMode_SEND,
						ResponseBodyMode:   v3alpha.ProcessingMode_BUFFERED,
					},
				}

			} else {
				resp = &pb.ProcessingResponse{
					Response: &pb.ProcessingResponse_RequestHeaders{
						RequestHeaders: rhq,
					},
				}
			}

			break

		case *pb.ProcessingRequest_RequestBody:

			r := req.Request
			b := r.(*pb.ProcessingRequest_RequestBody)
			log.Printf("   RequestBody: %s", string(b.RequestBody.Body))
			log.Printf("   EndOfStream: %T", b.RequestBody.EndOfStream)
			if b.RequestBody.EndOfStream {

				bytesToSend := append(b.RequestBody.Body, []byte(` baaar `)...)
				resp = &pb.ProcessingResponse{
					Response: &pb.ProcessingResponse_RequestBody{
						RequestBody: &pb.BodyResponse{
							Response: &pb.CommonResponse{
								HeaderMutation: &pb.HeaderMutation{
									SetHeaders: []*core.HeaderValueOption{
										{
											Header: &core.HeaderValue{
												Key:   "Content-Length",
												Value: strconv.Itoa(len(bytesToSend)),
											},
										},
									},
								},
								BodyMutation: &pb.BodyMutation{
									Mutation: &pb.BodyMutation_Body{
										Body: bytesToSend,
									},
								},
							},
						},
					},
					ModeOverride: &v3alpha.ProcessingMode{
						ResponseHeaderMode: v3alpha.ProcessingMode_SEND,
						ResponseBodyMode:   v3alpha.ProcessingMode_BUFFERED,
					},
				}
			}
			break
		case *pb.ProcessingRequest_ResponseHeaders:

			log.Printf("pb.ProcessingRequest_ResponseHeaders %v \n", v)
			r := req.Request
			h := r.(*pb.ProcessingRequest_ResponseHeaders)

			log.Printf("Got Response.Headers %v", h.ResponseHeaders.Headers)
			log.Printf("Got Response.Attributes %v", h.ResponseHeaders.Attributes)

			log.Println("  Removing access-control-allow-* headers")
			log.Println("  Adding x-server header")
			rhq := &pb.HeadersResponse{
				Response: &pb.CommonResponse{
					HeaderMutation: &pb.HeaderMutation{
						RemoveHeaders: []string{"access-control-allow-origin", "access-control-allow-credentials"},
						SetHeaders: []*core.HeaderValueOption{
							{
								Header: &core.HeaderValue{
									Key:   "x-server",
									Value: "envoy-proxy",
								},
							},
						},
					},
				},
			}
			resp = &pb.ProcessingResponse{
				Response: &pb.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: rhq,
				},
				//ModeOverride: &v3alpha.ProcessingMode{
				//	ResponseBodyMode: v3alpha.ProcessingMode_BUFFERED,
				//},
			}
			break
		case *pb.ProcessingRequest_ResponseBody:
			log.Printf("pb.ProcessingRequest_ResponseBody %v \n", v)
			r := req.Request
			b := r.(*pb.ProcessingRequest_ResponseBody)
			if b.ResponseBody.EndOfStream {
				bytesToSend := append(b.ResponseBody.Body, []byte(` pubci`)...)
				resp = &pb.ProcessingResponse{
					Response: &pb.ProcessingResponse_ResponseBody{
						ResponseBody: &pb.BodyResponse{
							Response: &pb.CommonResponse{
								BodyMutation: &pb.BodyMutation{
									Mutation: &pb.BodyMutation_Body{
										Body: bytesToSend,
									},
								},
							},
						},
					},
				}
			}

			break
		default:
			log.Printf("Unknown Request type %v\n", v)
		}
		if err := srv.Send(resp); err != nil {
			log.Printf("send error %v", err)
		}
	}
}

func main() {

	flag.Parse()

	lis, err := net.Listen("tcp", *grpcport)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	sopts := []grpc.ServerOption{grpc.MaxConcurrentStreams(1000)}
	s := grpc.NewServer(sopts...)

	pb.RegisterExternalProcessorServer(s, &server{})
	healthpb.RegisterHealthServer(s, &healthServer{})

	log.Printf("Starting gRPC server on port %s\n", *grpcport)

	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		log.Printf("caught sig: %+v", sig)
		log.Println("Wait for 1 second to finish processing")
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
	s.Serve(lis)
}
