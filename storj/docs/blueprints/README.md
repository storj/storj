# Blueprint Process

Design docs are now known as blueprints. We do not intend to keep blueprints up to date once
the implementation has been completed. The final step of a blueprint is to update documentation
elsewhere, such as:
 * https://github.com/storj/docs
 * https://documentation.storj.io
 * https://documentation.tardigrade.io
 * https://godoc.org/storj.io/storj/lib/uplink
 * Other godoc documentation, etc.

Blueprints will be checked in as Markdown files in `docs/blueprints` folder.

Blueprints should have an editor, at least one discussion meeting, and at least two reviewers from the architecture board.

The editor is responsible for:
* soliciting authors or authoring the document themselves,
* scheduling discussion meetings,
* finding reviewers,
* posting the document in our [forum](https://forum.storj.io/c/engineer-amas/design-draft), and
* getting the document finalized and merged

The editor is also responsible for making a list of epics and tickets after the blueprint is finalized and merged. These should include archiving the blueprint when completed and updating appropriate documentation.

The reviewers are responsible for reviewing the clarity and reasonableness of the document.

The discussion meeting should have at least three people present. Invitees should familiarize with the document prior to the discussion. The reviewers should be present. If there are open problems after the meeting, then the meeting should be repeated.

One of the reviewers must be an architecture owner, currently:

* Egon (@egonelbre),
* Kaloyan (@kaloyan-raev),
* JT (@jtolds),
* Jens (@littleskunk),
* unless agreed otherwise

The other reviewer should be someone with significant distributed systems expertise, currently:

* Paul (@thepaul),
* Simon (@simongui),
* Jeff (@zeebo),
* Matt (@brimstone),
* unless agreed otherwise.

However, it is expected that there should be feedback from the engineering team, DevOps team, data science team, UX team, QA team, and the community.

## Template

The blueprint uses `TEMPLATE.md` as a guide. However, it can be modified when necessary.

The template has the following sections:

* **Abstract** gives an overview of the blueprint and what it accomplishes. It does not go into details about the problem.

* **Background** gives an overview of the background why this blueprint exists. It describes the problems the blueprint tries to solve. It describes the goals of the design.

* **Design** contains the solution and its parts. The level of detail should correspond to the level of risk. The more problems a wrong solution would cause, the more detailed should be the description. The design should describe the solution to the degree it is usable by the end-user.

* **Rationale** section describes the alternate approaches and trade-offs. It should be clear why the proposed design was chosen among the alternate solutions.

* **Implementation** describes the steps to complete this blueprint. It should contain a rough outline of tasks. If necessary, there can be additional details about the changes to the codebase and process.

* **Wrap up** describes the steps needed

* **Open Issues** contains open questions that the author did not know how to solve.

## Format

The blueprint is intended to be read by developer contributors outside of Storj, but not the wider public. It is okay to assume some familiarity with the Storj project. Try to avoid acronyms that aren't common in the general developer community. Prefer simple sentences. Active voice is usually easier to read. Overall, be clear.
