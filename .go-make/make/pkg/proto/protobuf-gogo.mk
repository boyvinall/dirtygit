#
#  To use gogo, you need to define the following in your makefile
#  BEFORE including gomake:
#
#    PROTOC_GO_OUT:=--gofast_out=$(PROTOC_PARAMS):$(GOPATH)/src
#

export PROTOC_GEN_GOFAST:=$(GOPATH)/bin/protoc-gen-gofast

$(PROTOC_GEN_GOFAST):
	$(call PROMPT,Installing $@)
	go get github.com/gogo/protobuf/protoc-gen-gofast

tools:: $(PROTOC_GEN_GOFAST)

clean-tools::
	rm -f $(PROTOC_GEN_GOFAST)
