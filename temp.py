import csv
import random
import time

def generate_dummy_data(filename="dummy_data.csv", num_rows=1000000):
    headers = ["id", "name", "age", "email", "city", "score"]
    cities = ["New York", "London", "Tokyo", "Paris", "Berlin", "Sydney"]
    names = ["Alice", "Bob", "Charlie", "Diana", "Edward", "Fiona"]

    print(f"Generating {num_rows} rows... this might take a few seconds.")
    start_time = time.time()

    with open(filename, mode="w", newline="") as file:
        writer = csv.writer(file)
        writer.writerow(headers)

        for i in range(1, num_rows + 1):
            row = [
                i,
                random.choice(names),
                random.randint(18, 80),
                f"user{i}@example.com",
                random.choice(cities),
                round(random.uniform(0, 100), 2)
            ]
            writer.writerow(row)

    end_time = time.time()
    print(f"Success! File '{filename}' created in {end_time - start_time:.2f} seconds.")

if __name__ == "__main__":
    generate_dummy_data()