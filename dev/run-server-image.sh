
#!/usr/bin/env bash

#
# As per the apache version 2.0 license, this is a notice
# to let you, the reader, know that I have modified this file
# from its' original state.
#

# This runs a published or local server image.
# docker run pulls image if not found locally
IMAGE=${IMAGE:-sourcegraph/server:${TAG:-insiders}}
echo "starting server ${IMAGE}"
docker run "$@" \
  -d \
  --publish 7080:7080 \
  --publish 127.0.0.1:3370:3370 \
  --rm \
  --volume ~/.sourcegraph/config:/etc/sourcegraph \
  --volume ~/.sourcegraph/data:/var/opt/sourcegraph \
  "$IMAGE"
