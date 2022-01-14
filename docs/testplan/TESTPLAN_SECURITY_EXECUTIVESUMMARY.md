# CI Pipeline for Security Testing

## Static Application Security Testing (SAST)

These tools analyze source code to look for flaws unintentionally implemented which can create security issues as well as stability issues. These types of tests are often ran during the linting stage and are executed against changesets that are nearing a pull request. For our Golang code we will be using Gosec and/or Semgrep but we will also need tools that focus on our web frameworks as most attacks start at these sources.

### Software Composition Analysis (SCA)

#### Dependency Scanning

Dependency scanning is one of the most important aspects of SCA tests and is most commonly implemented during the linting process of the CI Pipeline. Since we are importing and implementing many third party libraries, we need to make sure that we aren’t a catalyst to carry through known security issues that those outside libraries invoke. To be proactive about these issues, we will leverage Gochk in the CI/CD pipeline to verify and maintain all of the external resources we use.

#### Container Scanning

Dependencies aren’t the only place where third party libraries are heavily implemented. We also need to be cognizant of code running within the containers we are leveraging; as well as any cluster workloads and environments that are being executed. Small parts of Starboard will be implemented to scan and verify containers/pods for the initial testing phase where we will execute test cases by hand. For the purpose of pipeline container scanning we will use the tools listed below.
1. Grype
2. Tri3vy 

#### Infrastructure as Code (IaC) Scanner

Even tools such as Ansible, Terraform, and AWS Cloud have known security issues and vulnerabilities. Keeping infrastructure as code secure (KICS) will be our tool of choice in this phase due to its robust solution and extensive feature set, some of the benefits are listed below.

1. Robust and simple architecture allows for quick additions to support new IaC solutions.
2. Contains over 2000 fully customizable and adjustable heuristic rules as well as the ability to create custom rules.
3. KICS is easy to install and run, easy to understand results, and easy to integrate into CI.

#### Secret Detection

Gitleaks will be used to scan the repository for any identifiable secrets based on the default ruleset as well as some hints that we provide to improve the possibility of detecting secrets.

### Dynamic Application Security Testing (DAST)

Most applications depend on multiple services, such as databases and caching services amongst other custom services based on the applications intended function. Dynamic tests occur on the compiled and running application code. These black box security tests attack the application by attempting to penetrate via exposed interfaces and endpoints. The main advantage from running DAST tests is that we can identify runtime issues quickly as well as find server configuration issues before a production release. Many different tools will likely need to be leveraged as different services within the repo will require unique test strategies a few listed below.


#### Guided Fuzz Tests

Guided fuzz testing on protobuf will be executed by using a mutator for LibFuzzer. Although LibFuzzer requires no corpus entries to begin fuzzing, it's important that we create valid and invalid entries. Doing so has a few essential advantages and ensures speed and accuracy. With both pass and fail starting entries we will have a baseline to verify that the fuzzer works as expected. Which helps us to build confidence in the test method and implementation. This will also help to train the fuzzer to build a more robust corpus in a shorter period of time. Just like anything else a well trained corpus will be able to find bugs that are more meaningful in less time.

#### Web API Fuzz Tests

GraphQL Endpoints: Web API Fuzz tests will be implemented against our GraphQL implementation on the satellite as this is a very likely entry for malicious sources. Using the saved configuration files from the Postman tests we created to generate the GraphQL Functional tests we are able to create the configuration files necessary for the fuzzer to get started. The few endpoints that we were unable to implement via postman configuration file were implemented by feeding the fuzzer a .har file generated using the developer tools from Chrome.


# Post Test Automation


## Monitoring and Alerting

Threat monitoring is a requirement from a security perspective. No matter what kind of protection or how many security checks you have in place there are no amount of tests that can ever prepare you against all attacks. There will always be zero day issues found and attackers who are slightly ahead of the industry. It's ok if you get compromised, nothing/no-one is perfect, what's not ok is not having a disaster recovery plan or not having contingency plans in place that allow for a quick resolution if a problem should arise.


### Vulnerability Reports, CVE Requests and Severity Levels Identification

Using tools like Vulcan and Remedy Cloud we can be proactive about vulnerabilities discovered in  code that we implement and code that we import. These tools are important because they can:


* Prioritize the requirement of the fix based on how we are leveraging third party libraries.
* Remediate the problem by providing a quick fix or work around depending on the development life-cycle.
* Analyze the resolution and verify that the vulnerability has been resolved as part of the process. 


### Third Party Security Scanner

Once our pipeline is in place it would be in our best interest to have an impartial third party do a security audit or to have someone run tools such as burp suite against the code base. In the event that a third party solution cannot be budgeted or requisitioned then the students																													

Revision Date: 01/10/2022
