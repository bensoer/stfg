package models

const EmbeddingModel = "Qwen/Qwen3-Embedding-0.6B-GGUF"

// 32768 is Qwen's max, but since were only generating embeddings for couple words up to a sentence.
// 1024 is plenty. This also should save on users memory usage
const EmbeddingModelContextSize = 1024
