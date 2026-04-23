package model

// ResultStream is a domain-level streaming interface replacing grpc.ServerStreamingClient.
// Implementations return io.EOF when the stream is exhausted.
type ResultStream[T any] interface {
	Recv() (T, error)
}
