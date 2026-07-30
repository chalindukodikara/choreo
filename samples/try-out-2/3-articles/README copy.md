# Editorial Brief for the Workflow Article

This article is being prepared as a Medium-style engineering post. Before it is ready for publishing, the draft in `TASK.md` needs a full copy rewrite so it reads like a polished technical blog post instead of a rough set of notes.

## Required length

The final article must contain at least 2,500 words of prose.

Do not count YAML blocks toward this minimum. YAML examples can stay in the article, but the article must still have at least 2,500 non-YAML words around them. That means the body text must carry the article through explanation, context, tradeoffs, and conclusions.

## Main objective

Explain how OpenChoreo Workflows were designed, what problems they solve, and why the design is useful for both platform engineers and developers.

The reader should come away understanding these ideas clearly:

- OpenChoreo uses a single workflow model for CI, provisioning, and other automation tasks.
- `Workflow` is the reusable definition.
- `WorkflowRun` is the execution of that definition.
- The design separates platform-owned concerns from developer-provided concerns.
- The workflow plane exists to run automation in a structured and scalable way.
- Parameters, CEL expressions, resources, and external references make the model flexible and reusable.

## What the rewrite must fix

- The title is incomplete and needs to be rewritten.
- The introduction is too rough and should start with the platform problem, not with scattered questions.
- Several sections read like internal notes rather than a finished article.
- Grammar, spelling, capitalization, and sentence flow need a full cleanup.
- The narrative needs stronger transitions between sections.
- The architecture section needs real explanation instead of just naming a diagram.
- The article currently leans too much on YAML. The rewrite must explain the ideas in prose first, then use YAML only to support the explanation.
- The conclusion is missing and must be added.

## Content expectations

Because the article must be at least 2,500 words without counting YAML, the rewrite needs more than sentence cleanup. It needs richer explanation.

Expand the article with:

- A stronger introduction that frames the workflow problem in an internal developer platform.
- A clear explanation of the design goals and why those goals matter.
- More detail on why separating `Workflow` and `WorkflowRun` is useful.
- A better explanation of how different personas interact with the system.
- A practical explanation of the workflow plane and where it fits in the architecture.
- Plain-language walkthroughs after each major YAML example.
- A fuller example of how a workflow definition becomes a workflow execution.
- A conclusion that ties the design choices back to real platform benefits.

## Writing style

- Use simple, direct, natural English.
- Prefer normal sentences over dense technical wording.
- Keep paragraphs reasonably short.
- Explain concepts before showing implementation details.
- Avoid repeating the same point in slightly different words.
- Do not sound like product marketing.
- Write for engineers who may not know OpenChoreo internals.

## Recommended structure

1. Title
2. Introduction
3. Why workflows matter in an internal developer platform
4. Design goals
5. High-level architecture
6. Why OpenChoreo uses `Workflow` and `WorkflowRun`
7. Separating platform engineer and developer concerns
8. Understanding the workflow plane
9. Core building blocks
10. Example using the Docker build workflow
11. Tradeoffs and benefits
12. Conclusion

## Guidance for YAML usage

YAML should support the article, not replace it.

- Keep only the YAML that is necessary for explanation.
- Trim overly long examples when a smaller excerpt makes the same point.
- After every YAML block, explain in plain language what the reader should notice.
- Do not rely on YAML blocks to satisfy the article length requirement.

## How to use the reference blogs

Reference files:

- `samples/try-out-2/3-articles/ref-1.MD`
- `samples/try-out-2/3-articles/ref-2.md`

Use these references for writing quality and structure only.

Borrow these patterns:

- Start with a clear problem and explain why it matters.
- Introduce design principles before deep implementation details.
- Move from high-level context to technical specifics in a controlled way.
- Keep the narrative flowing from problem to design to outcome.
- Use section headings that help the reader keep track of the argument.
- Mix explanation and technical detail carefully, without turning the article into raw documentation.

## Practical rewrite instructions

- Replace awkward lines like "how I can build my source code and deploy it?" with natural prose.
- Replace vague phrases like "we came up with 2 CRs" with clear architectural reasoning.
- Correct spelling issues such as `seperate` and `archtecture`.
- Use consistent capitalization for OpenChoreo and consistent naming for personas.
- Explain why each major concept exists, not just what it is.
- Add enough prose to exceed the 2,500-word minimum without depending on YAML.

## Acceptance criteria

The rewrite is successful only if all of the following are true:

- The article reads like a complete engineering blog post.
- The prose is clear, natural, and easy to follow.
- The structure is logical and reader-focused.
- The technical design is explained in plain language.
- The article contains at least 2,500 words excluding YAML blocks.
- The YAML examples are supportive, not dominant.
