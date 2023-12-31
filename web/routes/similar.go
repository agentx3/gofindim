package routes

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/agentx3/gofindim/data"
	"github.com/agentx3/gofindim/utils"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
)

var fieldsToFetch = []string{"path", "name", "id"}

func SimilarHandler(w http.ResponseWriter, r *http.Request) {
	weaviateClient := r.Context().Value("weaviateClient").(*weaviate.Client)
	var (
		results *[]data.ImageNode
		err     error
	)
	results = &[]data.ImageNode{}
	if weaviateClient == nil {
		http.Error(w, "Weaviate client not found", http.StatusInternalServerError)
		return
	}
	if r.Method == "POST" {
		results, err = similiarPostHandler(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	} else if r.Method == "GET" {
		results, err = similarGetHandler(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	} else if r.Method == "DELETE" {
		succeededUUIDs, err := similarDeleteHandler(w, r)
		json.NewEncoder(w).Encode(map[string]interface{}{"deleted_images": succeededUUIDs})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			http.StatusText(http.StatusOK)
		}
		return
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.Header.Add(w.Header(), "content-type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"images": results})
	http.StatusText(http.StatusOK)

	// c.JSON(200, gin.H{
	// 	"images": results,
	// })

}

func similiarPostHandler(w http.ResponseWriter, r *http.Request) (*[]data.ImageNode, error) {
	var (
		err     error
		results *[]data.ImageNode
	)
	weaviateClient := r.Context().Value("weaviateClient").(*weaviate.Client)
	// Retrieve the text input
	distance, err := utils.StringToFloat32(r.PostFormValue("distance"))
	if err != nil {
		fmt.Println("Error parsing distance")
		return nil, err
	}

	text_input := r.PostFormValue("text_input")

	text_weight, err := utils.StringToFloat32(r.PostFormValue("text_weight"))
	if err != nil {
		text_weight = 0.5
	}
	image_weight, err := utils.StringToFloat32(r.PostFormValue("image_weight"))
	if err != nil {
		image_weight = 0.5
	}
	limitStr := r.PostFormValue("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return nil, err
	}
	limit = int(math.Max(float64(1), float64(limit)))

	if textInput := r.PostFormValue("text_input"); textInput != "" {
		if path := r.PostFormValue("path"); path != "" {
			image, err := data.NewImageFileFromPath(path)
			if err != nil {
				fmt.Println("Error parsing image")
				return nil, err
			}
			results, err := data.SearchWeaviateWithTextAndImage(text_input,
				image,
				text_weight,
				image_weight,
				distance,
				limit,
				fieldsToFetch,
				weaviateClient,
			)
			if err != nil {
				return nil, err
			}
			return results, nil
		}
		fmt.Printf("Searching with text %v", textInput)
		if strings.HasPrefix(textInput, "http") {
			imageName := path.Base(textInput)
			imageFile, err := data.NewImageFileFromURL(textInput, imageName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			results, err = data.SearchWeaviateWithImageFile(
				imageFile,
				distance,
				limit,
				fieldsToFetch,
				weaviateClient,
			)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		} else {
			fmt.Printf("Searching with text %s", textInput)
			results, err = data.SearchWeaviateWithText(
				textInput,
				distance,
				limit,
				fieldsToFetch,
				weaviateClient,
			)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}

	} else if _, header, err := r.FormFile("file_input"); err == nil {
		// Retrieve the file from the form data
		results, err = data.SearchWeaviateWithFormFile(header, distance, limit, fieldsToFetch, weaviateClient)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}

	} else {
		w.WriteHeader(http.StatusBadRequest)
	} // Return a response
	return results, err
}

func similarGetHandler(w http.ResponseWriter, r *http.Request) (*[]data.ImageNode, error) {
	path := r.URL.Query().Get("path")
	text_input := r.URL.Query().Get("text_input")

	text_weight, err := utils.StringToFloat32(r.URL.Query().Get("text_weight"))
	if err != nil {
		text_weight = 0.5
	}

	image_weight, err := utils.StringToFloat32(r.URL.Query().Get("image_weight"))
	if err != nil {
		image_weight = 0.5
	}

	distance, err := utils.StringToFloat32(r.URL.Query().Get("distance"))
	if err != nil {
		distance = 0.8
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 10
	}

	uuid := r.URL.Query().Get("uuid")
	weaviateClient := r.Context().Value("weaviateClient").(*weaviate.Client)
	if uuid != "" {

		if text_input != "" {
			results, err := similarUUIDWithTextHandler(text_input, uuid, text_weight, image_weight, distance, limit, weaviateClient)
			if err != nil {
				return nil, err
			}
			return results, nil
		}

		results, err := similarUUIDHandler(uuid, distance, limit, weaviateClient)
		if err != nil {
			return nil, err
		}
		return results, nil

	} else if path != "" && text_input != "" {

		image, err := data.NewImageFileFromPath(path)
		if err != nil {
			fmt.Println("Error parsing image")
			return nil, err
		}
		results, err := data.SearchWeaviateWithTextAndImage(
			text_input,
			image,
			text_weight,
			image_weight,
			distance,
			limit,
			fieldsToFetch,
			weaviateClient,
		)

		if err != nil {
			fmt.Println("Error searching weaviate")
			return nil, err
		}

		return results, nil

	} else if path != "" {
		results, err := similarPathHandler(path, distance, limit, weaviateClient)
		if err != nil {
			fmt.Println("Error searching weaviate for similar images using only path", err)
			return nil, err
		}
		return results, nil
	}
	return nil, nil
}

func similarPathHandler(
	path string,
	distance float32,
	limit int,
	weaviateClient *weaviate.Client,
) (*[]data.ImageNode, error) {

	results, err := data.SearchWeaviateWithImagePath(path, distance, limit, fieldsToFetch, weaviateClient)
	if err != nil {
		return nil, err
	}
	return results, nil

}

func similarUUIDHandler(
	uuid string,
	distance float32,
	limit int,
	weaviateClient *weaviate.Client,
) (*[]data.ImageNode, error) {

	results, err := data.SearchWeaviateWithUUID(uuid, distance, limit, fieldsToFetch, weaviateClient)
	if err != nil {
		return nil, err
	}
	return results, nil

}

func similarUUIDWithTextHandler(
	text_input,
	uuid string,
	text_weight,
	image_weight,
	distance float32,
	limit int,
	weaviateClient *weaviate.Client,
) (*[]data.ImageNode, error) {

	results, err := data.SearchWeaviateWithTextAndUUID(
		text_input,
		uuid,
		text_weight,
		image_weight,
		distance,
		limit,
		fieldsToFetch,
		weaviateClient,
	)
	return results, err

}

func similarDeleteHandler(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	r.ParseMultipartForm(32 << 20)
	uuids := r.PostForm["delete_images[]"]
	paths := r.PostForm["delete_images_path[]"]
	var err error
	succeededUUIDs := []string{}
	weaviateClient := r.Context().Value("weaviateClient").(*weaviate.Client)
	fmt.Println("Deleting images with uuids", uuids)
	if len(uuids) > 0 {
		for i, uuid := range uuids {
			_err := data.DeleteWeaviateWithUUID(r.Context(), weaviateClient, uuid, paths[i])
			if _err != nil {
				err = _err
				fmt.Println("Error deleting image with uuid", uuid, err)
			} else {
				succeededUUIDs = append(succeededUUIDs, uuid)
			}
		}
	}
	return succeededUUIDs, err
}
