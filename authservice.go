package main

import (
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo"
	"github.com/penutty/authservice/user"
	"io/ioutil"
	"net/http"
	"reflect"
	"time"
)

func main() {
	e := echo.New()
	e.POST("/user/:user/create", createUser)
	e.POST("/auth", authUser)

	e.Logger.Fatal(e.Start(":8080"))
}

// CreateUserReq represents the fields and datatypes
// that are required by the createUser endpoint.
type createUserReq struct {
	authUserReq
	Email string
}

// createUser is a POST endpoint that accepts
// Content-Type: [application/json; charset=UTF-8]
// Body: {
//			UserID: UserID
//			Email: Email
//			Password: Password
//		 }
// on success returns
// Status: 201 - Created
func createUser(c echo.Context) error {
	resource := reflect.ValueOf(new(createUserReq)).Elem()
	err := validateContext(resource, c)
	if err != nil {
		c.Logger().Printf("main.ValidateContext Failed with err: %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	u := &user.User{
		AuthCredentials: user.AuthCredentials{
			UserID:   c.FormValue("UserID"),
			Password: c.FormValue("Password"),
		},
		Email: c.FormValue("Email"),
	}

	status := http.StatusCreated
	if err = user.CreateUser(u); err != nil {
		c.Logger().Printf("user.CreateUser Failed with error: %v", err)
		switch err {
		case user.UserAlreadyExists:
			status = http.StatusConflict
		default:
			status = http.StatusInternalServerError
		}
	}

	return c.NoContent(status)
}

// AuthCredentialsReq represents the fields and datatypes
// that are required by the authUser endpoint
type authUserReq struct {
	UserID   string
	Password string
}

// authUser is a POST endpoint that accepts
// Body: {
//			UserID: UserID
//			Password: Password
//		 }
// on success returns
// Status: 200
func authUser(c echo.Context) error {
	resource := reflect.ValueOf(new(authUserReq)).Elem()
	if err := validateContext(resource, c); err != nil {
		c.Logger().Printf("main.ValidateContext Failed with err: %v", err)
		return c.NoContent(http.StatusBadRequest)
	}

	aC := &user.AuthCredentials{
		UserID:   c.FormValue("UserID"),
		Password: c.FormValue("Password"),
	}

	if err := user.AuthUser(aC); err != nil {
		c.Logger().Printf("user.AuthUser Failed with error: %v", err)
		return c.NoContent(http.StatusUnauthorized)
	}

	token, err := generateJwt(c.FormValue("UserID"))
	if err != nil {
		c.Logger().Print(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	c.Response().Header().Set("jwt", token)

	return c.NoContent(http.StatusOK)
}

var ReqLengthStructLengthNotEqual = errors.New("Number of fields in request is different than number of struct fields.")
var ReqFieldsStructFieldsNotEqual = errors.New("Not all Request fields and resource fields are matching.")

// validateRequest compares the c Context of the request to the resource type
// that will be used to access the data.
// If c context has the incorrect number of fields, error.
// If c context does not have the correct fields, error.
// On success, return nil.
func validateContext(resource reflect.Value, c echo.Context) (err error) {

	fields := getResourceFields(resource, c)

	reqForm, err := c.FormParams()
	if err != nil {
		c.Logger().Printf("validateRequest Failed with error: %v", err)
		return err
	}

	if len(reqForm) != len(fields) {
		c.Logger().Printf("validateRequest failed, reqForm length = %v and %v length = %v.", len(reqForm), resource.String(), len(fields))
		return ReqLengthStructLengthNotEqual
	}

	for _, v := range fields {
		if stringValue := c.FormValue(v); stringValue == "" {
			c.Logger().Printf("Request was missing key:value pairs.")
			return ReqFieldsStructFieldsNotEqual
		}
	}

	return nil
}

var StructNotRecognized = errors.New("Arguement resource string not recognized past into getStructFields.")

// getResourceFields returns a string slice of all fields in argument resource.
// c Context is passed in for logging.
func getResourceFields(resource reflect.Value, c echo.Context) (rFields []string) {

	for i := 0; i < resource.NumField(); i++ {
		fieldValue := resource.Field(i)
		fieldName := resource.Type().Field(i).Name
		if fieldValue.Type().Kind() == reflect.Struct {
			recFields := getResourceFields(fieldValue, c)
			rFields = append(rFields, recFields...)
		} else {
			rFields = append(rFields, fieldName)
		}
	}

	return rFields
}

// generateJwt uses a requests UserID and a []byte secret to generate a JSON web token.
func generateJwt(UserID string) (string, error) {

	p, err := ioutil.ReadFile("/home/tjp/.ssh/jwt_private.pem")
	if err != nil {
		return "", err
	}

	t := jwt.New(jwt.SigningMethodRS256)
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return "", err
	}

	claims["iss"] = "Auth-Service"
	claims["sub"] = UserID
	claims["aud"] = "Moment-Service"
	claims["exp"] = time.Now().UTC().AddDate(0, 0, 7).Unix()
	claims["iat"] = time.Now().UTC().Unix()

	key, err := jwt.ParseRSAPrivateKeyFromPEM(p)
	if err != nil {
		return "", err
	}

	token, err := t.SignedString(key)
	if err != nil {
		return "", err
	}

	return token, nil
}
