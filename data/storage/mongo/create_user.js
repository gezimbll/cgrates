db = db.getSiblingDB('cgrates')
db.createUser(
  {
    user: "cgrates",
    pwd: "eVdyYzJRSjQW2Rec",
    roles: [ { role: "dbAdmin", db: "cgrates" } ]
  }
)

