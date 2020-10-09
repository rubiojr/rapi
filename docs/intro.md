# Intro

This guide is intended for those interested in learning how restic works internally.

You'll need some basic Go knowledge, a UNIX environment (macOS, Linux, etc) where Go and some command line tools are available and maybe some patience to deal with the author's mistakes, grammatical errors, misunderstandings and bugs.

Throughout this guide, I'll try to:

* Help you to understand how restic manages your data (security, portability, safety, etc), so you can make informed decisions if something happens to it, you want to move it around or alter it.
* Provide a building block for new projects. I've provided a simpler API (that comes from restic's source code for the most part), to help you with this.
* Provide working code examples to ilustrate how packs, blobs, keys, indices and other files are created and used.

The guide itself is being developed in a Linux/Ubuntu laptop, but any modern UNIX environment where Go is available should work.

Restic 0.10 (released on September 2020) and it's source at the time of the 0.10 release was used to develop the guide and the API.
