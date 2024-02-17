
docker.build:
	cd .. && docker build -t freyr/captain -f svc_captain/Dockerfile .

docker.push:
	docker tag freyr/captain australia-southeast2-docker.pkg.dev/freyr-operator/imgs/captain:latest
	docker push australia-southeast2-docker.pkg.dev/freyr-operator/imgs/captain:latest