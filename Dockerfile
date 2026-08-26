# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

FROM gcr.io/distroless/static-debian11:nonroot

ARG TARGETOS
ARG TARGETARCH

COPY bin/rel/docforge-${TARGETOS}-${TARGETARCH} /docforge

WORKDIR /

ENTRYPOINT [ "/docforge" ]
