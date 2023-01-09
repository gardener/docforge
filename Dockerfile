# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

FROM gcr.io/distroless/static-debian11:nonroot

ARG TARGETOS
ARG TARGETARCH

COPY bin/rel/docforge-${TARGETOS}-${TARGETARCH} /docforge

WORKDIR /

ENTRYPOINT [ "/docforge" ]
