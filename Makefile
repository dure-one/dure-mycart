.PHONY: update help

help:
	@echo "Available targets:"
	@echo "  make update    - Merge changes from fork/main_dure (keeps local changes on conflicts)"

update:
	@echo "Fetching from fork/main_dure..."
	@git fetch fork main_dure
	@echo "Merging (keeping local changes on conflicts)..."
	@git merge -X ours fork/main_dure || (echo "Conflicts detected. Resolving by keeping local changes..." && \
		git checkout --ours . && \
		git add . && \
		git commit -m "merge: resolve conflicts keeping local changes" && \
		echo "Merge completed with local changes preserved.")
	@echo "Update complete!"
