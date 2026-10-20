# Initial
I need you to make an application for a Quality Management Review system. It will have projects, each projects will have a set of questions, and each question will have action items created for them.
Projects would be in a list view, when clicked will bring to a project page.
A project page would display all questions. When a question is clicked, a side panel opens with fields to enter or edit answers. Each question should have the option to add an action item associated with that project's question. 
There should be a view of all questions of all projects with ability to filter/search that view.
There should be a view of all action items for all questions and projects with the ability to filter/search.

Users created should get a default temperatory password and be required to change the password on first log in. Password resets from an admin would trigger another required password change on temp password login.

In the start folder there is an excel file with a template to reference with table headers and dropdown options for some question fields.

Ask me questions to clarify workflow.

Use the logo file in the Web folder for any logo instances.
Use the Start/UI_Theme_Example.png for how the UI should look.
Backend written in GO.
Frontend written in vanilla html, css, javascript.
All wrapped into a docker container that will be deployed with komodo.

# Second
Items to change:
- When i reset a password for someone, a new temporary password is not displayed.
- An instructions section needs to be present. something like a wiki to put general instructions. It should have sections and subsections that expand/collapse on the same page. All of the fields are in the excel sheet.
    - One for an overview
    - One for instructions on each field
    - A scoring guide
    - A source matrix for reference material
- For project details I need sections for:
    - BAC Labor Revenue ($), numeric input
    - BAC Labor Cost ($), numeric input
    - BAC Labor Margin (%), calculated, 1 - (BAC Labor Revenue / BAC Labor Cost)
    - ACT LAB Margin (%), numeric input
    - Labor Margin Variance (%), calculated, ACT LAB Margin - BAC Labor Margin
- For tasks created on a question, I want to see tasks on the bottom when i open a question to edit. Also when adding a task, it should save the answers (now it closes the question without saving.)
- I want the tasks to be displayed in the same table as the project questions, intented under the respective question on the review page
- I want the total time for each review to be displayed on the review history table for a project.
- On questions, I need a checkbox for 'PM Clarification Used'. When checked, bring up a PM Clarification Summary text field.
- On questions, add a Checkbox for 'Follow-Up Required'

# Updates
[x] All questions main table:
    - take out evidence
    - add root cause with filter
[x] projects page dropdown for business unit

# next prompt
- Need an export of all data to put into AI to analyze patterns
    - Projects, questions, actions
- in all questions, when clicking a question, open the question in the side bar without going to project review page
    - add tasks under the all questions, like in the project review page