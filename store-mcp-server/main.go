package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

/* -------------------- helpers -------------------- */

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

/* -------------------- JSON-RPC types -------------------- */

type JSONRPCRequest struct {
	JsonRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

/* -------------------- MCP protocol types -------------------- */

type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Sampling     map[string]interface{} `json:"sampling,omitempty"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type string `json:"type"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []Content `json:"content"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

/* -------------------- MCP server -------------------- */

type MCPServer struct {
	initialized bool
	mu          sync.RWMutex
}

func NewMCPServer() *MCPServer {
	return &MCPServer{}
}

func (s *MCPServer) sendError(id interface{}, code int, msg string) JSONRPCResponse {
	return JSONRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: msg,
		},
	}
}

func (s *MCPServer) handleRequest(req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {

	case "initialize":
		s.mu.Lock()
		s.initialized = true
		s.mu.Unlock()

		return JSONRPCResponse{
			JsonRPC: "2.0",
			ID:      req.ID,
			Result: InitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: ServerCapabilities{
					Tools: &ToolsCapability{ListChanged: false},
				},
				ServerInfo: ServerInfo{
					Name:    "store-mcp-server",
					Version: "0.002",
				},
			},
		}

	case "tools/list":
		return JSONRPCResponse{
			JsonRPC: "2.0",
			ID:      req.ID,
			Result: ToolsListResult{
				Tools: []Tool{
					{
						Name:        "list_indian_stores",
						Description: "List popular Indian online stores",
						InputSchema: InputSchema{Type: "object"},
					},
				},
			},
		}

	case "tools/call":
		return JSONRPCResponse{
			JsonRPC: "2.0",
			ID:      req.ID,
			Result: CallToolResult{
				Content: []Content{
					{
						Type: "text",
						Text: "Flipkart, Amazon India, Reliance Digital, Myntra, Snapdeal, Tata CLiQ",
					},
				},
			},
		}

	case "ping":
		return JSONRPCResponse{
			JsonRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]string{},
		}

	default:
		return s.sendError(req.ID, -32601, "Method not found")
	}
}

/* -------------------- HTTP handlers -------------------- */

func rootHandler(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("hey there 👋 this is store-mcp-server"))
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func oauthMetadataHandler(w http.ResponseWriter, _ *http.Request) {
	meta := map[string]interface{}{
		"issuer":                 getenv("OAUTH_ISSUER", ""),
		"authorization_endpoint": getenv("OAUTH_AUTHORIZATION_ENDPOINT", ""),
		"token_endpoint":         getenv("OAUTH_TOKEN_ENDPOINT", ""),
		"jwks_uri":               getenv("OAUTH_JWKS_URI", ""),
		"response_types_supported": []string{
			"code",
		},
		"grant_types_supported": []string{
			"authorization_code",
		},
		"scopes_supported": strings.Split(
			getenv("OAUTH_SCOPES", "openid profile email"),
			" ",
		),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

func (s *MCPServer) mcpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(JSONRPCResponse{
			JsonRPC: "2.0",
			Error:   &RPCError{Code: -32700, Message: "Parse error"},
		})
		return
	}

	json.NewEncoder(w).Encode(s.handleRequest(req))
}

/* -------------------- main -------------------- */

func main() {
	log.Println("store-mcp-server starting on :8080")

	server := NewMCPServer()

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/.well-known/oauth-authorization-server", oauthMetadataHandler)
	http.HandleFunc("/mcp", server.mcpHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
