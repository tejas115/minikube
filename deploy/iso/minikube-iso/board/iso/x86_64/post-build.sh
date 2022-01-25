#!/bin/sh

# Copyright 2021 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

BOARD_DIR=$(dirname "$0")

if [ -e "$BOARD_DIR/iso/x86_64/grub.cfg" ]
then
    echo "ABC 1A"
else
    echo "ABC 1B"
fi

cp -f "$BOARD_DIR/iso/x86_64/grub.cfg" "$BINARIES_DIR/efi-part/EFI/BOOT/grub.cfg"

if [ -e "$BOARD_DIR/grub.cfg" ]
then
    echo "ABC 2A"
else
    echo "ABC 2B"
fi

cp -f "$BOARD_DIR/grub.cfg" "$BINARIES_DIR/efi-part/EFI/BOOT/grub.cfg"