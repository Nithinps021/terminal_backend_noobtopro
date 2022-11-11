#STEP 1 BUILD
#Specifying the base image.
FROM golang:1.19-alpine AS build

#creating a working directory
WORKDIR /app

#copying the go modules and dependencies
COPY go.mod ./

#downloading Go modules
RUN go mod download

#copying other go files
COPY ./ ./

#compile application
RUN go build -o terminal-backend

#STEP 2 DEPLOY
FROM nithinps021/terminal-server-allutil:v0.0.1

# adding user 
RUN addgroup -S appgroup && adduser -S noobtopro -G appgroup
USER noobtopro
WORKDIR /home/noobtopro/code
RUN chmod -R 777  /home/noobtopro/code

#Copying binary file from the build image
COPY --from=build /app/terminal-backend /app/terminal-backend
COPY assets /app/assets
EXPOSE 8080
ENTRYPOINT [ "/app/terminal-backend" ]
