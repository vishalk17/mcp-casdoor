package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

//
// --------------------
// JSON-RPC structures
// --------------------
//

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

//
// --------------------
// MCP protocol structs
// --------------------
//

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
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
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
	IsError bool      `json:"isError,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

//
// --------------------
// MCP server
// --------------------
//

type MCPServer struct {
	initialized bool
	mu          sync.RWMutex
}

func NewMCPServer() *MCPServer {
	return &MCPServer{}
}

func (s *MCPServer) sendError(id interface{}, code int, message string, data interface{}) JSONRPCResponse {
	return JSONRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func (s *MCPServer) handleRequest(req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {

	case "initialize":
		return s.handleInitialize(req.ID, req.Params)

	case "notifications/initialized":
		return JSONRPCResponse{}

	case "tools/list":
		if !s.isInitialized() {
			return s.sendError(req.ID, -32002, "Server not initialized", nil)
		}
		return s.handleToolsList(req.ID)

	case "tools/call":
		if !s.isInitialized() {
			return s.sendError(req.ID, -32002, "Server not initialized", nil)
		}
		return s.handleCallTool(req.ID, req.Params)

	case "ping":
		return JSONRPCResponse{
			JsonRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]string{},
		}

	default:
		return s.sendError(req.ID, -32601, "Method not found", req.Method)
	}
}

func (s *MCPServer) isInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

func (s *MCPServer) handleInitialize(id interface{}, params json.RawMessage) JSONRPCResponse {
	var initParams InitializeParams
	if err := json.Unmarshal(params, &initParams); err != nil {
		return s.sendError(id, -32602, "Invalid params", err.Error())
	}

	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	return JSONRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{ListChanged: false},
			},
			ServerInfo: ServerInfo{
				Name:    "indian-store-mcp-server",
				Version: "1.0.0",
			},
		},
	}
}

func (s *MCPServer) handleToolsList(id interface{}) JSONRPCResponse {
	return JSONRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
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
}

func (s *MCPServer) handleCallTool(id interface{}, params json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []Content{
				{Type: "text", Text: "Flipkart, Amazon India, Reliance Digital, Myntra, Snapdeal, Tata CLiQ"},
			},
		},
	}
}

//
// --------------------
// HTTP handlers
// --------------------
//

func (s *MCPServer) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
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

func healthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

//
// --------------------
// OAuth discovery (Casdoor)
// --------------------
//

func oauthAuthorizationServerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	casdoorBase := "https://casdoor.cloudwithme.dev"

	metadata := map[string]interface{}{
		"issuer":                                casdoorBase,
		"authorization_endpoint":                casdoorBase + "/login/oauth/authorize",
		"token_endpoint":                        casdoorBase + "/api/login/oauth/access_token",
		"userinfo_endpoint":                     casdoorBase + "/api/userinfo",
		"introspection_endpoint":                casdoorBase + "/api/introspect",
		"jwks_uri":                              casdoorBase + "/.well-known/jwks.json",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"scopes_supported":                      []string{"openid", "profile", "email", "offline_access"},
		"subject_types_supported":               []string{"public"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

//
// --------------------
// main
// --------------------
//

func main() {
	server := NewMCPServer()

	http.HandleFunc("/mcp", server.handleMCPRequest)
	http.HandleFunc("/health", healthCheck)

	// ✅ OAuth discovery pointing to CASDOOR
	http.HandleFunc("/.well-known/oauth-authorization-server", oauthAuthorizationServerHandler)

	log.Println("MCP server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
