package models

import (
	"runtime"

	llama "github.com/tcpipuk/llama-go"
	"go.uber.org/zap"
)

func CreateGroceryEmbedding(grocery string) ([]float32, error) {

	modelLocalAbsolutePath, err := ResolveModel(EmbeddingModel)
	if err != nil {
		zap.S().Error("Error Resolving Embedding Model")
		return nil, err
	}

	model, err := llama.LoadModel(modelLocalAbsolutePath,
		llama.WithGPULayers(-1),
		llama.WithMMap(true),
		llama.WithSilentLoading(),
	)
	if err != nil {
		zap.S().Error("Error Loading Model")
		return nil, err
	}
	defer model.Close()

	// Create context with embedding support
	ctx, err := model.NewContext(
		llama.WithContext(EmbeddingModelContextSize),
		llama.WithThreads(runtime.NumCPU()),
		llama.WithEmbeddings(),
		llama.WithF16Memory(),
	)
	if err != nil {
		zap.S().Error("Error Creating Context")
		return nil, err
	}
	defer ctx.Close()

	//fmt.Printf("Model loaded successfully.\n")
	//fmt.Printf("Getting embeddings for: %s\n", *text)

	embeddings, err := ctx.GetEmbeddings(grocery)
	if err != nil {
		zap.S().Errorf("Error Generating Embeddings For Grocery: %s", grocery)
		return nil, err
	}

	return embeddings, nil
}
