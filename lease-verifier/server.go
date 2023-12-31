package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "strconv"
    "time"

    "github.com/ethereum/go-ethereum/common"
    "github.com/iden3/go-circuits/v2"
    auth "github.com/iden3/go-iden3-auth/v2"
    "github.com/iden3/go-iden3-auth/v2/loaders"
    "github.com/iden3/go-iden3-auth/v2/pubsignals"
    "github.com/iden3/go-iden3-auth/v2/state"
    "github.com/iden3/iden3comm/v2/protocol"
    "github.com/gorilla/mux"
)

func main() {
    r := mux.NewRouter() 

    r.HandleFunc("/api/sign-in", GetAuthRequest)
    r.HandleFunc("/api/callback", Callback)

    // Enable CORS middleware
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow requests from any origin
			w.Header().Set("Access-Control-Allow-Origin", "*")
			
			// Allow common HTTP methods
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")

			// Allow certain headers
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// Continue processing the request
			next.ServeHTTP(w, r)
		})
	}

    http.ListenAndServe(":8080", corsMiddleware(r))
}

// Create a map to store the auth requests and their session IDs
var requestMap = make(map[string]interface{})

func GetAuthRequest(w http.ResponseWriter, r *http.Request) {

    // Audience is verifier id
    rURL := "https://7315-108-75-174-15.ngrok-free.app"
    sessionID := 1
    CallbackURL := "/api/callback"
    Audience := "did:polygonid:polygon:mumbai:2qDyy1kEo2AYcP3RT4XGea7BtxsY285szg6yP9SPrs"

    uri := fmt.Sprintf("%s%s?sessionId=%s", rURL, CallbackURL, strconv.Itoa(sessionID))

    // Generate request for basic authentication
    var request protocol.AuthorizationRequestMessage = auth.CreateAuthorizationRequest("test flow", Audience, uri)

    request.ID = "7f38a193-0918-4a48-9fac-36adfdb8b542"
    request.ThreadID = "7f38a193-0918-4a48-9fac-36adfdb8b542"

    // Add request for a specific proof
    var mtpProofRequest protocol.ZeroKnowledgeProofRequest
    mtpProofRequest.ID = 1
    mtpProofRequest.CircuitID = string(circuits.AtomicQuerySigV2CircuitID)
    mtpProofRequest.Query = map[string]interface{}{
        "allowedIssuers": []string{"*"},
        "credentialSubject": map[string]interface{}{
            "credit_score": map[string]interface{}{
                "$gt": 700,
            },
        },
        "context": "ipfs://QmSnFyKjeuD4FSaxXjcryLDj1VD9DDezA7p8N1kfXTZ7ei",
        "type":    "customSchema",
    }
    request.Body.Scope = append(request.Body.Scope, mtpProofRequest)

    // Store auth request in map associated with session ID
    requestMap[strconv.Itoa(sessionID)] = request

    // print request
    fmt.Println(request)

    msgBytes, _ := json.Marshal(request)

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(msgBytes)
    return
}

// Callback works with sign-in callbacks
func Callback(w http.ResponseWriter, r *http.Request) {

    // Get session ID from request
    sessionID := r.URL.Query().Get("sessionId")

    // get JWZ token params from the post request
    tokenBytes, _ := io.ReadAll(r.Body)

    // Add Polygon Mumbai RPC node endpoint - needed to read on-chain state
    ethURL := "https://polygon-testnet-rpc.allthatnode.com:8545"

    // Add IPFS url - needed to load schemas from IPFS 
    ipfsURL := "https://ipfs.io"

    // Add identity state contract address
    contractAddress := "0x134B1BE34911E39A8397ec6289782989729807a4"

    resolverPrefix := "polygon:mumbai"

    // Locate the directory that contains circuit's verification keys
    keyDIR := "../keys"

    // fetch authRequest from sessionID
    authRequest := requestMap[sessionID]

    // print authRequest
    fmt.Println(authRequest)

    // load the verifcation key
    var verificationKeyloader = &loaders.FSKeyLoader{Dir: keyDIR}
    resolver := state.ETHResolver{
        RPCUrl:          ethURL,
        ContractAddress: common.HexToAddress(contractAddress),
    }

    resolvers := map[string]pubsignals.StateResolver{
        resolverPrefix: resolver,
    }

    // EXECUTE VERIFICATION
    verifier, err := auth.NewVerifier(verificationKeyloader, resolvers, auth.WithIPFSGateway(ipfsURL))
    if err != nil {
        log.Println(err.Error())
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    authResponse, err := verifier.FullVerify(
        r.Context(),
        string(tokenBytes),
        authRequest.(protocol.AuthorizationRequestMessage),
        pubsignals.WithAcceptedStateTransitionDelay(time.Minute*5))
    if err != nil {
        log.Println(err.Error())
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    userID := authResponse.From

    messageBytes := []byte("User with ID " + userID + " Successfully authenticated")

    w.WriteHeader(http.StatusOK)
    w.Header().Set("Content-Type", "application/json")
    w.Write(messageBytes)

    return
}

