tag := `git tag --points-at HEAD | head -n 1`
sha := `git rev-parse --short HEAD`
version := if tag != "" { tag } else { sha }
build_date := `date -u '+%Y-%m-%dT%H:%M:%SZ'`
docker_image := "gcr.io/trois-six/k8s-diagrams"
output_dir := env("OUTPUT_DIR", "diagrams")
namespace := env("NAMESPACE", "traefikee")
viewer := env("VIEWER", "feh")

# Build the binary, and render the diagram of the namespace.
default: build render

# Lint the code.
check:
    golangci-lint run

# Remove the rendered diagrams.
clean:
    rm -rf {{ output_dir }}

# Build the binary.
build:
    @echo Version: {{ version }} {{ build_date }}
    CGO_ENABLED=0 go build -trimpath -ldflags '-X "main.version={{ version }}" -X "main.commit={{ sha }}" -X "main.date={{ build_date }}"'

# Run the tests.
test:
    go test -race -cover ./...

# Render the diagram of the namespace as diagram.png, passing extra flags such as --storage.
render *flags: clean
    ./k8s-diagrams -n {{ namespace }} -d {{ output_dir }} {{ flags }}
    cd {{ output_dir }} && dot -Tpng k8s.dot > ../diagram.png

# Build, render and display the diagram of the namespace.
display *flags: build (render flags)
    {{ viewer }} diagram.png

# Build the Docker image.
image:
    docker build --build-arg VERSION={{ version }} --build-arg COMMIT={{ sha }} --build-arg DATE={{ build_date }} \
        -t {{ docker_image }}:{{ version }} .

# Push the Docker image.
publish:
    docker push {{ docker_image }}:{{ version }}

# Push the Docker image as latest.
publish-latest: publish
    docker tag {{ docker_image }}:{{ version }} {{ docker_image }}:latest
    docker push {{ docker_image }}:latest
