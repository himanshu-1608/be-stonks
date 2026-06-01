Initiate a git repo for this folder.
Consider this as the starting of this repo.

I will start with my first requirement:
Consider that this is the backend of a stock trading related activities.
It will have zerodha kite api interaction. Kite API documentation is present at:
https://kite.trade/docs/connect/v3/

I want to start with a simple integration of alerts.
My requirement is that I will have a recommendation csv, like this:
/Users/hy/Documents/projects/personal-portfolio/input/recommendation.csv
Think of the folder structure where you want it in this repo.

- You need to create 3 alerts for each stock recommendation: First when it hits target 1, second when it is in exactly middle between target 1 and 2 (mid-hit), and 3rd alert is when it hits target 2.
- The alerts name should be consistent so that i dont have to explicitly remember.
- It should be on LTP only
- It should be NSE-based not BSE-based
- It should be only alert, no ATO is required for now
- When you create an alert, create an alert_mapping.csv file, so that you dont create duplicate alerts everyday.
- Before creating an alert, do a check whether that alert already exists or not in csv file
- This should be a daily diff activity, i will update the input recommendation csv file everday, and there will be a cron scheduled for everyday on the machine where this is working, for it to trigger everyday at 8:30 am , 4 pm.
- Think in terms of a backend /sync API only, because i will be creating other APIs in backend as well for this whole stock trading requirement like price ticks etc.
- The backend language you can decide. This will run on AWS EC2 instance FYI. So think accordingly.
