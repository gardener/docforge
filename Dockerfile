# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

FROM gcr.io/distroless/static-debian11:nonroot

COPY bin/rel/docforge-linux-amd64 /docforge

WORKDIR /

ENTRYPOINT [ "/docforge" ]
