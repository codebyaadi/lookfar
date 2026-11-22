# syntax=docker/dockerfile:1
ARG GO_VERSION=1.22
ARG BASE
ARG UNSET_TAG

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
RUN echo hi

FROM build AS test
RUN echo test

FROM node:20@sha256:abcdef AS web
FROM ghcr.io/org/app:2.1 AS app
FROM localhost:5000/team/tool:v1
FROM nginx
FROM ${BASE}
FROM redis:${UNSET_TAG}
FROM scratch
COPY --from=build /out /out
