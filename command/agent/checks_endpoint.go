package agent

const (
	clientChecksRPC = "Allocations.Checks"
)

//func (s *HTTPServer) ClientChecksEndpoint(resp http.ResponseWriter, req *http.Request) (any, error) {
//	// Get the requested Node ID
//	requestedNode := req.URL.Query().Get("node_id")
//
//	// Build the request and parse the ACL token
//	args := structs.NodeSpecificRequest{
//		NodeID: requestedNode,
//	}
//	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)
//
//	// Determine the handler to use
//	useLocalClient, useClientRPC, useServerRPC := s.rpcHandlerForNode(requestedNode)
//
//	// Make the RPC
//	var reply structs.CheckResultsByAllocationResponse
//	var rpcErr error
//	switch {
//	case useLocalClient:
//		rpcErr = s.agent.Client().ClientRPC(clientChecksRPC, &args, &reply)
//	case useClientRPC:
//		rpcErr = s.agent.Client().RPC(clientChecksRPC, &args, &reply)
//	case useServerRPC:
//		rpcErr = s.agent.Server().RPC(clientChecksRPC, &args, &reply)
//	default:
//		rpcErr = CodedError(400, "No local Node and node_id not provided")
//	}
//
//	if rpcErr != nil {
//		if structs.IsErrNoNodeConn(rpcErr) {
//			rpcErr = CodedError(404, rpcErr.Error())
//		} else if strings.Contains(rpcErr.Error(), "Unknown node") {
//			rpcErr = CodedError(404, rpcErr.Error())
//		}
//		return nil, rpcErr
//	}
//	return nil, nil
//}
