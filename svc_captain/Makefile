
docker.build:
	docker build -t freyr/captain .
	docker run --rm -it :latest

gcloud.push:
	docker tag freyr/captain australia-southeast2-docker.pkg.dev/freyr-operator/imgs/captain:latest
	docker push australia-southeast2-docker.pkg.dev/freyr-operator/imgs/captain:latest