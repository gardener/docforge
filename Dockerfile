# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

FROM alpine:3.11.6

COPY bin/rel/docforge-linux-amd64 /usr/local/bin/docforge

WORKDIR /
