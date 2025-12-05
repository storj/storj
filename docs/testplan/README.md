# How To Create a Basic Testplan!

&nbsp;

![](https://github.com/storj/storj/raw/main/resources/logo.png)


&nbsp;

### Why write Testplans?

We believe test plans are important, Testplans written as early as possible before implementation starts work like a checklist and as soon as we finish the implementation we can compare it to the test plan to check for any bugs. Also even before implementation is finished, if a developer can read a test plan beforehand they can prevent most bugs, hence why we believe that the earlier we write a test plan the more bugs we can prevent!

### Hi! Here is a guide on how to write a basic **testplan** that will go over how to

- figure out a **_test name_**

- group similar test cases into something that we can turn into a task (**_test scenarios_**)

- add a **_description_** of what the test does

- add **_comments_** related to the test, comments that add links to additional information f.e blueprints, design drafts (even if they are private) or even comments requesting a feature that relates to the test!

- take all this info and place it into a neat **testplan template**!

&nbsp;

&nbsp;


## Test Name

#### Let's say we have these tests in mind for the multinode dashboard UI

1. Seeing if creating a new node works with correct information

2. Seeing if creating a new node works with incorrect information

3. Seeing if creating a new node works with an already existing node with the same information



<!-- end of the list -->

Now we can just simplify these tests and create test names for these test cases, so 1, 2 and 3 become

1. Add new node with correct information

2. Add new node with incorrect information

3. Add new node with existing node information



## Test Scenarios

Using the aforementioned tests above we have these test cases in mind for the multinode dashboard UI



- Add new node with correct information

- Add new node with incorrect information

- Add new node with existing node information

&nbsp;



Here we can see that we are able to group up these testcases into a task or **test scenario**, since each of these tests fall under the **add a new node button** we can just label these tests under the scenario **New Node Button Functionality**. We would also add a **number ID** to make the test plan easier to read

&nbsp;



&nbsp;





## Test Description



Now building onto the test cases we can describe what should happen in each test **(EXPECTED OUTCOME)** case f.e using a conditional if-then statement. So we can take our previous test cases

1. Add new node with correct information

2. Add new node with incorrect information

3. Add new node with existing node information

<!-- end of the list -->



**_and add a conditional statement so we can know how to test it and what to look for to consider the test as passing or failing_**




1. If a user clicks on add a new node button and inputs **correct node** info, then user **should be able to see node ID, disk space used, disk space left, bandwidth used, earned currency, version and status of said node**



2. If a user clicks on add a new node button and inputs **incorrect node** info, then user **should not be able to see node ID, disk space used, disk space left, bandwidth used, earned currency, version, status of said node and instead a error message stating node information was incorrect**



3. If a user clicks on add a new node button and inputs **an existing node on their dashboard**, then the user **should just recieve an error message stating their node is already on the dashboard**

<!-- end of the list -->

>So if we conduct test 2, and see that **we aren't able to view the node ID, disk space used, disk spaced left, bandwidth used, earned currency, version, status of said node on the dashboard** and **_NO_** error message stating the node information was incorrect, then the test failed



&nbsp;



&nbsp;


## Comments



In this section of the testplan users can input their own **comments, why said tests were added, improvements needed and what the actual outcomes for said tests are** when it falls out of line towards expected outcomes f.e going back to **test case 2**. We see that although inputting incorrect node information doesn't allow us to see what the multinode dashboard usually allows us to see since the node information is incorrect in this case, we still weren't shown an **error message stating node information was incorrect**, so in this case the user can comment that the actual result was that although the user was not able to see information like the node ID, disk space used etc. the user was supposed to see the error message stating node information was incorrect but did not in this case





## Testplan Template and How to Submit Testplan

By now if you have followed along thus far you should have a pretty awesome high level testplan! Once you have replaced each category under each column the next step



```

| Test Scenario      | Test Case     | Test Description   | Comments     |

| :----------------  | :------------ | :---------------   | :----------  |

| Test Scenario 1    | Test Case 1.1 | TC1.1 Description  | TC1.1 Comment  |

|                    | Test Case 1.2 | TC1.2 Descriotion  | TC1.2 Comment  |

| Test Scenario 2    | Test Case 2.1 | TC2.1 Description  | TC2.1 Comment  |

|                    | Test Case 2.2 | TC2.2 Descriotion  | TC2.2 Comment  |

```

is to head to [testplan repo](https://github.com/storj/storj/tree/main/docs/testplan) and open a pull request titled **Test Plan** on said feature by [using our template here](TEMPLATE.md)! If you think you are done writing your awesome test plan, head to the testplan repo and open a pull request with an appropriate title containing the keywords, **Test Plan** along with the **Test Plan** label.

## Support



If you have any questions or suggestions please reach out to us on [our community forum](https://forum.storj.io/) or file a support ticket at [https://support.storj.io](https://support.storj.io/).

