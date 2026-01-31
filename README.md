start redis
docker run -d -p 6379:6379 redis:alpine
docker ps -a
docker stop <container_id>
docker rm <container_id>