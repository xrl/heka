#!/bin/sh
set -ex

if [[ $1 == "" ]]; then
  echo "usage: release_travis.sh DOCKER_REPO_SLUG"
  exit 1
fi
DOCKER_REPO_SLUG = $1
IMAGE = "${DOCKER_REPO_SLUG}:${TRAVIS_BRANCH}"

cd $TRAVIS_BUILD_DIR
mkdir /tmp/heka
find . -name "*.deb" -exec cp {} /tmp/heka/heka.deb \;
cp docker/Dockerfile.travis /tmp/heka/Dockerfile

docker build -t $IMAGE /tmp/heka
docker login -e="$DOCKER_EMAIL" -u="$DOCKER_USERNAME" -p="$DOCKER_PASSWORD"
docker push $IMAGE