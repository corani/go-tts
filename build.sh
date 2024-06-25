#!/bin/bash
prog=$(realpath "$0")
rc=0

function print_usage {
  echo "Usage: $prog [options...]"
  echo
  echo "Available options:"
  echo "  -h            prints this help"
  echo "  -b            build"
  echo "  -d            build docker"
  echo "  -r            run demo"
  echo "  -s            generate samples for all speakers"
  echo "  -m            download model"
}

function do_echo {
    echo "[CMD] $*"
    TIMEFORMAT="[INFO] took %3lR"
    time "$@"
    code=$?
    if [ $code -ne 0 ]; then
        rc=$code
        echo "[ERROR] return code $rc"
    fi
}

function log_info {
    # shellcheck disable=SC2145
    echo "[INFO] $@"
}

if [ "$#" -eq "0" ]; then
    print_usage
    exit 0
fi

function download_model {
    if [ ! -d vits-coqui-en-vctk ]; then
        log_info "downloading model"
        curl -SL -O https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/vits-coqui-en-vctk.tar.bz2
        tar xvf vits-coqui-en-vctk.tar.bz2
        rm vits-coqui-en-vctk.tar.bz2
    else 
        log_info "model already downloaded"
    fi
}

function generate_samples {
    # start docker, generate samples for all speakers (0-108), stop docker. 
    container_id=$(docker run -d --rm   \
        --tmpfs /tmp                    \
        -p 8383:8383                    \
        go-tts:latest --addr=:8383)
    trap 'docker stop $container_id' EXIT

    log_info "waiting for server to start"

    # wait until GET / return code 200
    until curl --output /dev/null --silent --head --fail http://localhost:8383; do
        sleep 1
    done

    log_info "server started, generating samples for all speakers"

    for SID in $(seq 0 108); do
        curl -H "Content-Type: application/json" \
            -d "{\"speaker\":${SID},\"text\":\"I am speaker ${SID}. The quick brown fox jumped over the lazy dog.\"}" \
            http://localhost:8383/> "out/generated-${SID}.wav"
    done

    log_info "all samples generated, stopping server"
}

function build_binary {
    if [ ! -d vits-coqui-en-vctk ]; then
        download_model
    fi
    do_echo go build -o bin/go-tts .
}

function build_docker {
    if [ ! -d vits-coqui-en-vctk ]; then
        download_model
    fi
    do_echo docker build -t go-tts . 
}

mkdir -p bin out

while [ "$#" -gt "0" ]; do
  arg=$1
  shift

  case $arg in
    -h)
        print_usage
        exit 0
        ;;
    -b)
        build_binary
        ;;
    -d)
        build_docker
        ;;
    -m)
        download_model
        ;;
    -r)
        if [ -z "$(docker images -q go-tts:latest 2> /dev/null)" ]; then 
            build_docker
        fi
        do_echo docker run                              \
            -v "$(pwd)/out:/out"                        \
            go-tts:latest                               \
                --output-filename=/out/generated.wav    \
                "All your bases are belong to us."
        ;;
    -s)
        if [ -z "$(docker images -q go-tts:latest 2> /dev/null)" ]; then 
            build_docker
        fi
        generate_samples
        ;;
    *)
        echo "[ERROR] unrecognized argument '$arg'"
        print_usage
        exit 1
        ;;
  esac
done

exit "${rc}"

