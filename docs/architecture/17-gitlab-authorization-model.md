## 17. GitLab authorization model

Use GitLab's own security mechanisms in addition to zero runtime controls:

- protected branches;
- MR approvals;
- CODEOWNERS;
- CI validation;
- project/repository permission boundaries;
- runner isolation;
- registry permissions.

The agent should normally be able to:

```text
read issues/MRs/pipelines
push feature branches
create MRs
```

without automatically being able to:

```text
push main
force-push
merge protected MRs
cancel/retry arbitrary production pipelines
create releases/tags
change project settings
```

---

