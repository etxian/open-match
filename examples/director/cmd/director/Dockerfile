FROM open-match-base-build as builder

WORKDIR /go/src/open-match.dev/open-match/examples/director/cmd/director

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo .

FROM alpine:3.8

RUN adduser -D director
COPY --from=builder \
    --chown=director \
    /go/src/open-match.dev/open-match/examples/director/cmd/director/director \
    /home/director/director

USER director
WORKDIR /home/director
ENTRYPOINT ["/home/director/director"]
