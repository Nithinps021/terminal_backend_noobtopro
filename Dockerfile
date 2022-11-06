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
FROM alpine:3.14
WORKDIR /

#Copying binary file from the build image
COPY --from=build /app/terminal-backend ./terminal-backend
COPY assets .
EXPOSE 8080
CMD [ "/terminal-backend" ]
