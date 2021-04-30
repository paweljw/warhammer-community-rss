# builder stage
FROM golang:1.14 as builder
COPY . src
RUN rm -rf .git
ENV GO111MODULE=on
# ENV GOFLAGS=-mod=vendor
# RUN go get -u "github.com/gorilla/feeds"
# RUN go get -u "github.com/PuerkitoBio/goquery"
# RUN go get -u "github.com/chromedp/cdproto/dom"
# RUN go get -u "github.com/chromedp/chromedp"
# RUN go get -u "github.com/robfig/cron/v3"
RUN cd src && go build \
  -ldflags "-linkmode external -extldflags -static" \
  -o server

# run stage
FROM chromedp/headless-shell:latest
COPY --from=builder /go/src/server ./server
RUN apt-get update
RUN apt install dumb-init
RUN mkdir static
ENTRYPOINT ["dumb-init", "--"]
EXPOSE 8100
CMD ["./server"]
