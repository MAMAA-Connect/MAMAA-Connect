package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	pb "server/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedMamaaBridgeServer
}

func (s *server) StreamUnifiedInference(req *pb.InferenceRequest, stream pb.MamaaBridge_StreamUnifiedInferenceServer) error {
	ctx := stream.Context()
	log.Printf("[MAMAA] Incoming request for Provider: %s", req.Provider)

	switch req.Provider {
	case "google":
		return s.handleGeminiStream(ctx, req, stream)
	case "microsoft":
		return s.handleOpenAIStream(ctx, req, stream)
	case "meta":
		return s.handleLlamaStream(ctx, req, stream)
	default:
		return status.Errorf(codes.InvalidArgument, "Unsupported provider: %s", req.Provider)
	}
}

func (s *server) handleGeminiStream(ctx context.Context, req *pb.InferenceRequest, stream pb.MamaaBridge_StreamUnifiedInferenceServer) error {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	url := fmt.Sprintf("https://googleapis.com", apiKey)

	payload := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"parts": []interface{}{
					map[string]interface{}{"text": req.Prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     req.Temperature,
			"maxOutputTokens": req.MaxTokens,
		},
	}

	return s.executeHttpRequest(ctx, "google", url, payload, stream)
}

func (s *server) handleOpenAIStream(ctx context.Context, req *pb.InferenceRequest, stream pb.MamaaBridge_StreamUnifiedInferenceServer) error {
	apiKey := os.Getenv("AZURE_OPENAI_API_KEY")
	url := os.Getenv("AZURE_OPENAI_ENDPOINT") // Ex: https://YOUR_://azure.com

	payload := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": req.Prompt},
		},
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      true,
	}

	return s.executeHttpRequest(ctx, "microsoft", url, payload, stream)
}

func (s *server) handleLlamaStream(ctx context.Context, req *pb.InferenceRequest, stream pb.MamaaBridge_StreamUnifiedInferenceServer) error {
	// Padrão Open-Source Llama via infraestrutura Groq ou Ollama local
	url := "https://groq.com"
	apiKey := os.Getenv("GROQ_API_KEY")

	payload := map[string]interface{}{
		"model": "llama3-8b-8192",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": req.Prompt},
		},
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      true,
	}

	return s.executeHttpRequest(ctx, "meta", url, payload, stream)
}

func (s *server) executeHttpRequest(ctx context.Context, provider string, url string, payload map[string]interface{}, stream pb.MamaaBridge_StreamUnifiedInferenceServer) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to marshal payload: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to create HTTP request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if provider == "microsoft" {
		httpReq.Header.Set("api-key", os.Getenv("AZURE_OPENAI_API_KEY"))
	} else if provider == "meta" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("GROQ_API_KEY")))
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return status.Errorf(codes.Unavailable, "HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var chunk map[string]interface{}
			if err := decoder.Decode(&chunk); err == io.EOF {
				return nil
			} else if err != nil {
				// Ignora fragmentos de formatação de stream SSE vazios de APIs de terceiros
				continue
			}

			// Extração dinâmica de texto simplificada para fins de demonstração arquitetural
			textChunk := fmt.Sprintf("%v", chunk) 

			err = stream.Send(&pb.InferenceResponse{
				Chunk:     textChunk,
				Timestamp: time.Now().UnixMilli(),
				ModelUsed: provider,
			})
			if err != nil {
				return err
			}
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMamaaBridgeServer(grpcServer, &server{})

	log.Println("[MAMAA-Connect] Edge Gateway running on port :50051...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
